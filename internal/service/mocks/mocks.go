package mocks

import (
	"context"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"
)

type MockEventStore struct {
	SaveEventsFn func(ctx context.Context, aggregateId string, events []payment.Event, expectedVersion int) error
	LoadEventsFn func(ctx context.Context, aggregateId string) ([]payment.Event, error)
}

func (m *MockEventStore) SaveEvents(ctx context.Context, aggregateId string, events []payment.Event, expectedVersion int) error {
	if m.SaveEventsFn != nil {
		return m.SaveEventsFn(ctx, aggregateId, events, expectedVersion)
	}
	return nil
}

func (m *MockEventStore) LoadEvents(ctx context.Context, aggregateId string) ([]payment.Event, error) {
	if m.LoadEventsFn != nil {
		return m.LoadEventsFn(ctx, aggregateId)
	}
	return nil, nil
}

type MockReadRepository struct {
	GetPaymentByIdFn func(ctx context.Context, id string) (*payment.Payment, error)
	UpsertFn         func(ctx context.Context, p *payment.Payment) error
}

func (m *MockReadRepository) GetPaymentById(ctx context.Context, id string) (*payment.Payment, error) {
	if m.GetPaymentByIdFn != nil {
		return m.GetPaymentByIdFn(ctx, id)
	}
	return nil, nil
}

func (m *MockReadRepository) Upsert(ctx context.Context, p *payment.Payment) error {
	if m.UpsertFn != nil {
		return m.UpsertFn(ctx, p)
	}
	return nil
}

type MockPaymentProvider struct {
	AuthorizeFn func(ctx context.Context, p payment.Payment) error
}

func (m *MockPaymentProvider) Authorize(ctx context.Context, p payment.Payment) error {
	if m.AuthorizeFn != nil {
		return m.AuthorizeFn(ctx, p)
	}
	return nil
}

type MockPublisher struct {
	PublishEventFn   func(ctx context.Context, event payment.Event) error
	PublishCommandFn func(ctx context.Context, cmd string) error
}

func (m *MockPublisher) PublishEvent(ctx context.Context, event payment.Event) error {
	if m.PublishEventFn != nil {
		return m.PublishEventFn(ctx, event)
	}
	return nil
}

func (m *MockPublisher) PublishCommand(ctx context.Context, cmd string) error {
	if m.PublishCommandFn != nil {
		return m.PublishCommandFn(ctx, cmd)
	}
	return nil
}

type MockCommandRepository struct {
	IsProcessedFn   func(ctx context.Context, commandID string) (bool, error)
	MarkProcessedFn func(ctx context.Context, commandID string, result port.CommandResult) error
}

func (m *MockCommandRepository) IsProcessed(ctx context.Context, commandID string) (bool, error) {
	if m.IsProcessedFn != nil {
		return m.IsProcessedFn(ctx, commandID)
	}
	return false, nil
}

func (m *MockCommandRepository) MarkProcessed(ctx context.Context, commandID string, result port.CommandResult) error {
	if m.MarkProcessedFn != nil {
		return m.MarkProcessedFn(ctx, commandID, result)
	}
	return nil
}
