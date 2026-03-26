package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(paymentHandler *PaymentHandler, metricsHandler http.Handler) chi.Router {
	r := chi.NewRouter()

	r.Use(TracingMiddleware)
	r.Use(MetricsMiddleware)

	r.Handle("/metrics", metricsHandler)

	r.Route("/payments", func(r chi.Router) {
		r.Post("/", paymentHandler.CreatePayment)
		r.Get("/{id}", paymentHandler.GetPayment)
		r.Post("/{id}/cancel", paymentHandler.CancelPayment)
		r.Post("/callback", paymentHandler.PaymentCallback)
	})

	return r
}
