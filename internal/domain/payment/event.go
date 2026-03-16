package payment

import "time"

type PaymentEvent string

const (
	PaymentCreated    PaymentEvent = "payment_created"
	ProcessingStarted PaymentEvent = "processing_started"
	PaymentCompleted  PaymentEvent = "payment_completed"
	PaymentFailed     PaymentEvent = "payment_failed"
	PaymentCanceled   PaymentEvent = "payment_canceled"
)

type Event struct {
	ID          int          `json:"id"`
	AggregateID string       `json:"aggregate_id"`
	Type        PaymentEvent `json:"type"`
	Version     int          `json:"version"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   time.Time    `json:"created_at"`
}
