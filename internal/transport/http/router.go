package http

import "github.com/go-chi/chi/v5"

func NewRouter(paymentHandler *PaymentHandler) chi.Router {
	r := chi.NewRouter()

	r.Route("/payments", func(r chi.Router) {
		r.Post("/", paymentHandler.CreatePayment)
		r.Get("/{id}", paymentHandler.GetPayment)
		r.Post("/{id}/cancel", paymentHandler.CancelPayment)
		r.Post("/callback", paymentHandler.PaymentCallback)
	})

	return r
}
