package http

import (
	"context"
	"encoding/json"
	"net/http"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/service"

	"github.com/go-chi/chi/v5"
)

type PaymentUseCase interface {
	CreatePayment(ctx context.Context, req *service.CreatePaymentRequest) (string, error)
	GetPayment(ctx context.Context, id string) (*payment.Payment, error)
	CancelPayment(ctx context.Context, id string) error
}

type PaymentHandler struct {
	useCase PaymentUseCase
}

func NewPaymentHandler(uc PaymentUseCase) *PaymentHandler {
	return &PaymentHandler{useCase: uc}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	appReq := &service.CreatePaymentRequest{
		IdempotencyKey: req.IdempotencyKey,
		Amount:         req.Amount,
		Currency:       req.Currency,
	}

	id, err := h.useCase.CreatePayment(r.Context(), appReq)
	if err != nil {
		status, msg := mapError(err)
		http.Error(w, msg, status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing payment ID", http.StatusBadRequest)
		return
	}

	p, err := h.useCase.GetPayment(r.Context(), id)
	if err != nil {
		status, msg := mapError(err)
		http.Error(w, msg, status)
		return
	}

	resp := PaymentResponse{
		ID:             p.ID,
		IdempotencyKey: p.IdempotencyKey,
		Amount:         p.Amount,
		Currency:       p.Currency,
		Status:         p.Status,
		Version:        p.Version,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *PaymentHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing payment ID", http.StatusBadRequest)
		return
	}

	err := h.useCase.CancelPayment(r.Context(), id)
	if err != nil {
		status, msg := mapError(err)
		http.Error(w, msg, status)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *PaymentHandler) PaymentCallback(w http.ResponseWriter, r *http.Request) {
	// TODO: обработка callback от psp
	w.WriteHeader(http.StatusOK)
}
