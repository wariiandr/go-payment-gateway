package service

import (
	"context"
	"fmt"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

var (
	tracer = otel.Tracer("payment-gateway/service")
	meter  = otel.Meter("payment-gateway/service")

	paymentStatusTotal, _ = meter.Int64Counter("payment_status_total",
		metric.WithDescription("Total payments by business status"),
	)
)

type PaymentService struct {
	eventStore port.EventStore
	readRepo   port.PaymentReadRepository
	provider   port.PaymentProvider
	publisher  port.PaymentPublisher
}

func NewPaymentService(eventStore port.EventStore, readRepo port.PaymentReadRepository, provider port.PaymentProvider, publisher port.PaymentPublisher) *PaymentService {
	return &PaymentService{
		eventStore: eventStore,
		readRepo:   readRepo,
		provider:   provider,
		publisher:  publisher,
	}
}

type CreatePaymentRequest struct {
	IdempotencyKey string           `json:"idempotency_key"`
	Amount         int64            `json:"amount"`
	Currency       payment.Currency `json:"currency"`
}

func (s *PaymentService) CreatePayment(ctx context.Context, request *CreatePaymentRequest) error {
	ctx, span := tracer.Start(ctx, "PaymentService.CreatePayment")
	defer span.End()

	span.SetAttributes(
		attribute.String("payment.currency", string(request.Currency)),
		attribute.Int64("payment.amount", request.Amount),
		attribute.String("payment.idempotency_key", request.IdempotencyKey),
	)

	p, err := payment.CreatePayment(request.IdempotencyKey, request.Amount, request.Currency)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetAttributes(attribute.String("payment.id", p.ID))

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), 0)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = s.publisher.PublishCommand(ctx, fmt.Sprintf("process_payment:%s", p.ID))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	s.recordPaymentStatus(ctx, string(payment.New), request.Currency)

	return nil
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

func (s *PaymentService) ProcessPayment(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "PaymentService.ProcessPayment")
	defer span.End()

	span.SetAttributes(attribute.String("payment.id", id))

	events, err := s.eventStore.LoadEvents(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	initialVersion := p.Version

	err = p.StartProcessing()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = s.provider.Authorize(ctx, *p)
	if err != nil {
		p.Fail()
		s.recordPaymentStatus(ctx, string(payment.Failed), p.Currency)
	} else {
		p.Complete()
		s.recordPaymentStatus(ctx, string(payment.Completed), p.Currency)
	}

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), initialVersion)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	for _, event := range p.UncommittedEvents() {
		err = s.publisher.PublishEvent(ctx, event)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}

	return nil
}

func (s *PaymentService) CancelPayment(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "PaymentService.CancelPayment")
	defer span.End()

	span.SetAttributes(attribute.String("payment.id", id))

	events, err := s.eventStore.LoadEvents(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	initialVersion := p.Version

	err = p.Cancel()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), initialVersion)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	for _, event := range p.UncommittedEvents() {
		err = s.publisher.PublishEvent(ctx, event)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = s.readRepo.Upsert(ctx, p)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
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
