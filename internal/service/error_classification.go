package service

import (
	"context"
	"errors"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	errTypeDomain    = "domain"
	errTypeDatabase  = "database"
	errTypeMessaging = "messaging"
	errTypeProvider  = "provider"
	errTypeContext   = "context"
	errTypeUnknown   = "unknown"
)

func classifyPaymentError(err error) string {
	if err == nil {
		return errTypeUnknown
	}
	if isDomainError(err) {
		return errTypeDomain
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errTypeDatabase
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return errTypeDatabase
	}
	if errors.Is(err, port.ErrMessaging) {
		return errTypeMessaging
	}
	if errors.Is(err, port.ErrProvider) {
		return errTypeProvider
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errTypeContext
	}
	return errTypeUnknown
}

func isDomainError(err error) bool {
	return errors.Is(err, payment.ErrInvalidAmount) ||
		errors.Is(err, payment.ErrInvalidCurrency) ||
		errors.Is(err, payment.ErrInvalidTransition) ||
		errors.Is(err, payment.ErrUnknownEventType) ||
		errors.Is(err, payment.ErrNoEvents) ||
		errors.Is(err, payment.ErrPaymentFailed) ||
		errors.Is(err, payment.ErrConcurrencyConflict)
}
