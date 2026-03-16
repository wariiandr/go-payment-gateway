package port

import (
	"context"
	"payment-gateway/internal/domain/payment"
)

type PaymentPublisher interface {
	PublishEvent(ctx context.Context, event payment.Event) error
	PublishCommand(ctx context.Context, cmd string) error
}
