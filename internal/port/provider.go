package port

import (
	"context"
	"payment-gateway/internal/domain/payment"
)

type PaymentProvider interface {
	Authorize(ctx context.Context, payment payment.Payment) error
}
