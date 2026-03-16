package port

import (
	"context"

	"payment-gateway/internal/domain/payment"
)

type EventStore interface {
	SaveEvents(ctx context.Context, aggregateId string, events []payment.Event, expectedVersion int) error
	LoadEvents(ctx context.Context, aggregateId string) ([]payment.Event, error)
}

type PaymentReadRepository interface {
	GetPaymentById(ctx context.Context, id string) (*payment.Payment, error)
	Upsert(ctx context.Context, payment *payment.Payment) error
}
