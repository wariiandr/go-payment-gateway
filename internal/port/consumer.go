package port

import "context"

type PaymentProcessor interface {
	ProcessPayment(ctx context.Context, commandID string, paymentID string) error
}

type PaymentProjection interface {
	Project(ctx context.Context, aggregateID string) error
}
