package service

import (
	"context"
	"fmt"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"
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
	p, err := payment.CreatePayment(request.IdempotencyKey, request.Amount, request.Currency)
	if err != nil {
		return err
	}

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), 0)
	if err != nil {
		return err
	}

	err = s.publisher.PublishCommand(ctx, fmt.Sprintf("process_payment:%s", p.ID))
	if err != nil {
		return err
	}

	return nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id string) (*payment.Payment, error) {
	return s.readRepo.GetPaymentById(ctx, id)
}

func (s *PaymentService) ProcessPayment(ctx context.Context, id string) error {
	events, err := s.eventStore.LoadEvents(ctx, id)
	if err != nil {
		return err
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		return err
	}

	initialVersion := p.Version

	err = p.StartProcessing()
	if err != nil {
		return err
	}

	err = s.provider.Authorize(ctx, *p)
	if err != nil {
		p.Fail()
	} else {
		p.Complete()
	}

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), initialVersion)
	if err != nil {
		return err
	}

	for _, event := range p.UncommittedEvents() {
		err = s.publisher.PublishEvent(ctx, event)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *PaymentService) CancelPayment(ctx context.Context, id string) error {
	events, err := s.eventStore.LoadEvents(ctx, id)
	if err != nil {
		return err
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		return err
	}

	initialVersion := p.Version

	err = p.Cancel()
	if err != nil {
		return err
	}

	err = s.eventStore.SaveEvents(ctx, p.ID, p.UncommittedEvents(), initialVersion)
	if err != nil {
		return err
	}

	for _, event := range p.UncommittedEvents() {
		err = s.publisher.PublishEvent(ctx, event)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *PaymentService) Project(ctx context.Context, aggregateID string) error {
	events, err := s.eventStore.LoadEvents(ctx, aggregateID)
	if err != nil {
		return err
	}

	p, err := payment.ReconstructFromEvents(events)
	if err != nil {
		return err
	}

	return s.readRepo.Upsert(ctx, p)
}
