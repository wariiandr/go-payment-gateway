package service

import (
	"context"
	"fmt"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("payment-gateway/service")
	meter  = otel.Meter("payment-gateway/service")

	paymentStatusTotal, _ = meter.Int64Counter("payment_status_total",
		metric.WithDescription("Total payments by business status"),
	)
	paymentsCreatedTotal, _ = meter.Int64Counter("payments_created_total",
		metric.WithDescription("Total number of payment aggregates created"),
	)
	paymentProcessingOutcomesTotal, _ = meter.Int64Counter("payment_processing_outcomes_total",
		metric.WithDescription("Outcomes of payment processing in the worker (after authorize)"),
	)
	paymentProcessingDuration, _ = meter.Float64Histogram("payment_processing_duration_seconds",
		metric.WithDescription("Wall time spent in ProcessPayment"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120),
	)
	paymentRetryAttemptsTotal, _ = meter.Int64Counter("payment_retry_attempts_total",
		metric.WithDescription("Extra authorize attempts after the first one"),
	)
	paymentErrorsTotal, _ = meter.Int64Counter("payment_errors_total",
		metric.WithDescription("Errors in payment service by coarse type"),
	)
)

type PaymentService struct {
	eventStore           port.EventStore
	readRepo             port.PaymentReadRepository
	provider             port.PaymentProvider
	publisher            port.PaymentPublisher
	commandRepo          port.CommandRepository
	authorizeMaxAttempts int
}

func NewPaymentService(
	eventStore port.EventStore,
	readRepo port.PaymentReadRepository,
	provider port.PaymentProvider,
	publisher port.PaymentPublisher,
	commandRepo port.CommandRepository,
	authorizeMaxAttempts int,
) *PaymentService {
	if authorizeMaxAttempts <= 0 {
		authorizeMaxAttempts = 1
	}

	return &PaymentService{
		eventStore:           eventStore,
		readRepo:             readRepo,
		provider:             provider,
		publisher:            publisher,
		commandRepo:          commandRepo,
		authorizeMaxAttempts: authorizeMaxAttempts,
	}
}

type CreatePaymentRequest struct {
	IdempotencyKey string           `json:"idempotency_key"`
	Amount         int64            `json:"amount"`
	Currency       payment.Currency `json:"currency"`
}

func (s *PaymentService) CreatePayment(ctx context.Context, request *CreatePaymentRequest) (string, error) {
	ctx, span := tracer.Start(ctx, "PaymentService.CreatePayment")
	defer span.End()

	span.SetAttributes(
		attribute.String("payment.currency", string(request.Currency)),
		attribute.Int64("payment.amount", request.Amount),
		attribute.String("payment.idempotency_key", request.IdempotencyKey),
	)

	p, err := payment.CreatePayment(request.IdempotencyKey, request.Amount, request.Currency)
	if err != nil {
		return "", s.failSpan(ctx, span, err)
	}

	span.SetAttributes(attribute.String("payment.id", p.ID))

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), 0)
	if err != nil {
		return "", s.failSpan(ctx, span, err)
	}

	commandID := uuid.New().String()
	span.SetAttributes(attribute.String("command.id", commandID))

	err = s.publisher.PublishCommand(ctx, fmt.Sprintf("process_payment:%s:%s", commandID, p.ID))
	if err != nil {
		return "", s.failSpan(ctx, span, err)
	}

	paymentsCreatedTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("payment.currency", string(request.Currency))))
	s.recordPaymentStatus(ctx, string(payment.New), request.Currency)

	return p.ID, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id string) (*payment.Payment, error) {
	ctx, span := tracer.Start(ctx, "PaymentService.GetPayment")
	defer span.End()

	span.SetAttributes(attribute.String("payment.id", id))

	p, err := s.readRepo.GetPaymentById(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return p, nil
}

func (s *PaymentService) ProcessPayment(ctx context.Context, commandID string, paymentID string) error {
	ctx, span := tracer.Start(ctx, "PaymentService.ProcessPayment")
	defer span.End()

	started := time.Now()
	defer func() {
		s.recordProcessPaymentDuration(ctx, time.Since(started).Seconds())
	}()

	span.SetAttributes(
		attribute.String("command.id", commandID),
		attribute.String("payment.id", paymentID),
	)

	alreadyProcessed, err := s.commandRepo.IsProcessed(ctx, commandID)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}
	if alreadyProcessed {
		span.SetAttributes(attribute.Bool("command.duplicate", true))
		return nil
	}

	events, err := s.eventStore.LoadEvents(ctx, paymentID)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	initialVersion := p.Version

	err = p.StartProcessing()
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	result := port.CommandResultCompleted

	var lastAuthErr error
	for attempt := 0; attempt < s.authorizeMaxAttempts; attempt++ {
		if attempt > 0 {
			s.recordPaymentRetry(ctx)
			backoff := time.Duration(50*(1<<uint(attempt-1))) * time.Millisecond
			select {
			case <-ctx.Done():
				return s.failSpan(ctx, span, ctx.Err())
			case <-time.After(backoff):
			}
		}

		lastAuthErr = s.provider.Authorize(ctx, *p)
		if lastAuthErr == nil {
			break
		}
	}

	if lastAuthErr != nil {
		err = p.Fail()
		if err != nil {
			return s.failSpan(ctx, span, err)
		}
		result = port.CommandResultFailed
		s.recordPaymentStatus(ctx, string(payment.Failed), p.Currency)
		s.recordProcessingOutcome(ctx, "failure", p.Currency)
	} else {
		err = p.Complete()
		if err != nil {
			return s.failSpan(ctx, span, err)
		}
		s.recordPaymentStatus(ctx, string(payment.Completed), p.Currency)
		s.recordProcessingOutcome(ctx, "success", p.Currency)
	}

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), initialVersion)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	for _, event := range p.UncommittedEvents() {
		err = s.publisher.PublishEvent(ctx, event)
		if err != nil {
			return s.failSpan(ctx, span, err)
		}
	}

	err = s.commandRepo.MarkProcessed(ctx, commandID, result)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	return nil
}

func (s *PaymentService) CancelPayment(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "PaymentService.CancelPayment")
	defer span.End()

	span.SetAttributes(attribute.String("payment.id", id))

	events, err := s.eventStore.LoadEvents(ctx, id)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	initialVersion := p.Version

	err = p.Cancel()
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), initialVersion)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	for _, event := range p.UncommittedEvents() {
		err = s.publisher.PublishEvent(ctx, event)
		if err != nil {
			return s.failSpan(ctx, span, err)
		}
	}

	s.recordPaymentStatus(ctx, string(payment.Canceled), p.Currency)

	return nil
}

func (s *PaymentService) Project(ctx context.Context, aggregateID string) error {
	ctx, span := tracer.Start(ctx, "PaymentService.Project")
	defer span.End()

	span.SetAttributes(attribute.String("aggregate.id", aggregateID))

	events, err := s.eventStore.LoadEvents(ctx, aggregateID)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	err = s.readRepo.Upsert(ctx, p)
	if err != nil {
		return s.failSpan(ctx, span, err)
	}

	return nil
}

func (s *PaymentService) recordPaymentStatus(ctx context.Context, status string, currency payment.Currency) {
	paymentStatusTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("payment.status", status),
			attribute.String("payment.currency", string(currency)),
		),
	)
}

func (s *PaymentService) recordProcessingOutcome(ctx context.Context, outcome string, currency payment.Currency) {
	paymentProcessingOutcomesTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("outcome", outcome),
			attribute.String("payment.currency", string(currency)),
		),
	)
}

func (s *PaymentService) recordProcessPaymentDuration(ctx context.Context, seconds float64) {
	paymentProcessingDuration.Record(ctx, seconds)
}

func (s *PaymentService) recordPaymentRetry(ctx context.Context) {
	paymentRetryAttemptsTotal.Add(ctx, 1)
}

func (s *PaymentService) recordPaymentError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	paymentErrorsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("error_type", classifyPaymentError(err)),
		),
	)
}

func (s *PaymentService) failSpan(ctx context.Context, span trace.Span, err error) error {
	s.recordPaymentError(ctx, err)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	return err
}
