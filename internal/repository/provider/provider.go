package provider

import (
	"context"
	"payment-gateway/internal/domain/payment"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("payment-gateway/repository/provider")

type PaymentProvider struct{}

func NewPaymentProvider() *PaymentProvider {
	return &PaymentProvider{}
}

func (p *PaymentProvider) Authorize(ctx context.Context, pay payment.Payment) error {
	ctx, span := tracer.Start(ctx, "PaymentProvider.Authorize")
	defer span.End()

	span.SetAttributes(
		attribute.String("payment.id", pay.ID),
		attribute.Int64("payment.amount", pay.Amount),
		attribute.String("payment.currency", string(pay.Currency)),
	)

	// TODO: заменить на реальный вызов psp
	time.Sleep(3 * time.Second)
	return nil
}
