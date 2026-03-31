package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/service"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUseCase struct {
	CreatePaymentFn func(ctx context.Context, req *service.CreatePaymentRequest) (string, error)
	GetPaymentFn    func(ctx context.Context, id string) (*payment.Payment, error)
	CancelPaymentFn func(ctx context.Context, id string) error
}

func (m *mockUseCase) CreatePayment(ctx context.Context, req *service.CreatePaymentRequest) (string, error) {
	if m.CreatePaymentFn != nil {
		return m.CreatePaymentFn(ctx, req)
	}
	return "pay-123", nil
}

func (m *mockUseCase) GetPayment(ctx context.Context, id string) (*payment.Payment, error) {
	if m.GetPaymentFn != nil {
		return m.GetPaymentFn(ctx, id)
	}
	return &payment.Payment{ID: id, Status: payment.Completed}, nil
}

func (m *mockUseCase) CancelPayment(ctx context.Context, id string) error {
	if m.CancelPaymentFn != nil {
		return m.CancelPaymentFn(ctx, id)
	}
	return nil
}

func setupRouter(uc PaymentUseCase) *chi.Mux {
	r := chi.NewRouter()
	h := NewPaymentHandler(uc)
	r.Post("/payments", h.CreatePayment)
	r.Get("/payments/{id}", h.GetPayment)
	r.Post("/payments/{id}/cancel", h.CancelPayment)
	return r
}

func TestCreatePayment_Success(t *testing.T) {
	uc := &mockUseCase{
		CreatePaymentFn: func(ctx context.Context, req *service.CreatePaymentRequest) (string, error) {
			return "pay-success-id", nil
		},
	}
	r := setupRouter(uc)

	body := `{"idempotency_key": "k-1", "amount": 100, "currency": "USD"}`
	req := httptest.NewRequest("POST", "/payments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusCreated, res.StatusCode)

	var resBody map[string]string
	err := json.NewDecoder(res.Body).Decode(&resBody)
	require.NoError(t, err)
	assert.Equal(t, "pay-success-id", resBody["id"])
}

func TestCreatePayment_InvalidJSON(t *testing.T) {
	uc := &mockUseCase{}
	r := setupRouter(uc)

	body := `{"amount": "not-a-number"}`
	req := httptest.NewRequest("POST", "/payments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

func TestCreatePayment_DomainError(t *testing.T) {
	uc := &mockUseCase{
		CreatePaymentFn: func(ctx context.Context, req *service.CreatePaymentRequest) (string, error) {
			return "", payment.ErrInvalidAmount
		},
	}
	r := setupRouter(uc)

	body := `{"idempotency_key": "k-1", "amount": -100, "currency": "USD"}`
	req := httptest.NewRequest("POST", "/payments", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.GreaterOrEqual(t, w.Result().StatusCode, 400)
}

func TestGetPayment_Success(t *testing.T) {
	uc := &mockUseCase{
		GetPaymentFn: func(ctx context.Context, id string) (*payment.Payment, error) {
			now := time.Now()
			return &payment.Payment{
				ID:             id,
				IdempotencyKey: "k-2",
				Amount:         500,
				Currency:       payment.RUB,
				Status:         payment.Processing,
				CreatedAt:      now,
				UpdatedAt:      now,
			}, nil
		},
	}
	r := setupRouter(uc)

	req := httptest.NewRequest("GET", "/payments/pay-123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)

	var p PaymentResponse
	err := json.NewDecoder(res.Body).Decode(&p)
	require.NoError(t, err)

	assert.Equal(t, "pay-123", p.ID)
	assert.Equal(t, payment.Processing, p.Status)
	assert.Equal(t, int64(500), p.Amount)
}

func TestGetPayment_NotFound(t *testing.T) {
	uc := &mockUseCase{
		GetPaymentFn: func(ctx context.Context, id string) (*payment.Payment, error) {
			return nil, errors.New("not found")
		},
	}
	r := setupRouter(uc)

	req := httptest.NewRequest("GET", "/payments/pay-999", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.GreaterOrEqual(t, w.Result().StatusCode, 400)
}

func TestCancelPayment_Success(t *testing.T) {
	uc := &mockUseCase{
		CancelPaymentFn: func(ctx context.Context, id string) error {
			return nil
		},
	}
	r := setupRouter(uc)

	req := httptest.NewRequest("POST", "/payments/p-1/cancel", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestCancelPayment_InvalidTransition(t *testing.T) {
	uc := &mockUseCase{
		CancelPaymentFn: func(ctx context.Context, id string) error {
			return payment.ErrInvalidTransition
		},
	}
	r := setupRouter(uc)

	req := httptest.NewRequest("POST", "/payments/p-1/cancel", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.GreaterOrEqual(t, w.Result().StatusCode, 400)
}
