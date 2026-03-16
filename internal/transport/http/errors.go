package http

import (
	"errors"
	"net/http"
	"payment-gateway/internal/domain/payment"
)

func mapError(err error) (int, string) {
	switch {
	case errors.Is(err, payment.ErrInvalidAmount):
		return http.StatusBadRequest, "Invalid amount"
	case errors.Is(err, payment.ErrInvalidCurrency):
		return http.StatusBadRequest, "Invalid currency"
	case errors.Is(err, payment.ErrInvalidTransition):
		return http.StatusConflict, "Invalid payment status transition"
	case errors.Is(err, payment.ErrNoEvents):
		return http.StatusNotFound, "Payment not found"
	case errors.Is(err, payment.ErrConcurrencyConflict):
		return http.StatusConflict, "Payment was modified by another request"
	default:
		return http.StatusInternalServerError, "Internal server error"
	}
}
