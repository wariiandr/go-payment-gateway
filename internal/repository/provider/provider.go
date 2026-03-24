package provider

import (
	"context"
	"payment-gateway/internal/domain/payment"
	"time"
)

type PaymentProvider struct{}

func NewPaymentProvider() *PaymentProvider {
	return &PaymentProvider{}
}

func (p *PaymentProvider) Authorize(ctx context.Context, pay payment.Payment) error {
	// TODO: заменить на реальный вызов psp
	time.Sleep(3 * time.Second)
	return nil
}
