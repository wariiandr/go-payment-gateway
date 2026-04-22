package service

import (
	"errors"
	"fmt"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestClassifyPaymentError(t *testing.T) {
	t.Parallel()

	t.Run("domain", func(t *testing.T) {
		assert.Equal(t, errTypeDomain, classifyPaymentError(payment.ErrInvalidAmount))
		assert.Equal(t, errTypeDomain, classifyPaymentError(fmt.Errorf("wrap: %w", payment.ErrNoEvents)))
	})

	t.Run("database", func(t *testing.T) {
		assert.Equal(t, errTypeDatabase, classifyPaymentError(pgx.ErrNoRows))
		pg := &pgconn.PgError{Code: "23505"}
		assert.Equal(t, errTypeDatabase, classifyPaymentError(pg))
	})

	t.Run("messaging", func(t *testing.T) {
		assert.Equal(t, errTypeMessaging, classifyPaymentError(fmt.Errorf("kafka: %w", port.ErrMessaging)))
	})

	t.Run("provider", func(t *testing.T) {
		assert.Equal(t, errTypeProvider, classifyPaymentError(fmt.Errorf("psp: %w", port.ErrProvider)))
	})

	t.Run("unknown", func(t *testing.T) {
		assert.Equal(t, errTypeUnknown, classifyPaymentError(nil))
		assert.Equal(t, errTypeUnknown, classifyPaymentError(errors.New("opaque")))
	})
}
