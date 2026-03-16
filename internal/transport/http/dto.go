package http

import (
	"payment-gateway/internal/domain/payment"
	"time"
)

type CreatePaymentRequest struct {
	IdempotencyKey string           `json:"idempotency_key"`
	Amount         int64            `json:"amount"`
	Currency       payment.Currency `json:"currency"`
}

type PaymentResponse struct {
	ID             string                `json:"id"`
	IdempotencyKey string                `json:"idempotency_key"`
	Amount         int64                 `json:"amount"`
	Currency       payment.Currency      `json:"currency"`
	Status         payment.PaymentStatus `json:"status"`
	Version        int                   `json:"version"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}
