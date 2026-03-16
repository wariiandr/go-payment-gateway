package payment

import (
	"time"

	"github.com/google/uuid"
)

type Payment struct {
	ID                string
	IdempotencyKey    string
	Amount            int64
	Currency          Currency
	Status            PaymentStatus
	Version           int
	CreatedAt         time.Time
	UpdatedAt         time.Time
	uncommittedEvents []Event
}

func CreatePayment(idempotencyKey string, amount int64, currency Currency) (*Payment, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	if !currency.IsValid() {
		return nil, ErrInvalidCurrency
	}

	p := &Payment{
		ID:                uuid.New().String(),
		IdempotencyKey:    idempotencyKey,
		Amount:            amount,
		Currency:          currency,
		Status:            New,
		CreatedAt:         time.Now(),
		uncommittedEvents: []Event{},
	}

	event := Event{
		AggregateID: p.ID,
		Type:        PaymentCreated,
		Version:     1,
		CreatedAt:   time.Now(),
		Payload: map[string]any{
			"idempotency_key": idempotencyKey,
			"amount":          amount,
			"currency":        currency,
		},
	}

	p.ApplyEvent(event)
	p.uncommittedEvents = append(p.uncommittedEvents, event)

	return p, nil
}

func (p *Payment) ApplyEvent(event Event) error {
	switch event.Type {
	case PaymentCreated:
		p.ID = event.AggregateID
		p.Status = New
		if v, ok := event.Payload["idempotency_key"].(string); ok {
			p.IdempotencyKey = v
		}
		if v, ok := event.Payload["amount"].(float64); ok {
			p.Amount = int64(v)
		}
		if v, ok := event.Payload["currency"].(string); ok {
			p.Currency = Currency(v)
		}
	case ProcessingStarted:
		p.Status = Processing
	case PaymentCompleted:
		p.Status = Completed
	case PaymentFailed:
		p.Status = Failed
	case PaymentCanceled:
		p.Status = Canceled
	default:
		return ErrUnknownEventType
	}

	p.Version++
	p.UpdatedAt = time.Now()

	return nil
}

func (p *Payment) transition(status PaymentStatus, eventType PaymentEvent) error {
	if !CanTransition(p.Status, status) {
		return ErrInvalidTransition
	}

	event := Event{
		AggregateID: p.ID,
		Type:        eventType,
		Version:     p.Version + 1,
		CreatedAt:   time.Now(),
		Payload: map[string]any{
			"amount":   p.Amount,
			"currency": p.Currency,
		},
	}

	p.ApplyEvent(event)

	p.uncommittedEvents = append(p.uncommittedEvents, event)

	return nil
}

func (p *Payment) StartProcessing() error {
	return p.transition(Processing, ProcessingStarted)
}

func (p *Payment) Complete() error {
	return p.transition(Completed, PaymentCompleted)
}

func (p *Payment) Fail() error {
	return p.transition(Failed, PaymentFailed)
}

func (p *Payment) Cancel() error {
	return p.transition(Canceled, PaymentCanceled)
}

func ReconstructFromEvents(events []Event) (*Payment, error) {
	if len(events) == 0 {
		return nil, ErrNoEvents
	}

	payment := &Payment{}

	for _, event := range events {
		if err := payment.ApplyEvent(event); err != nil {
			return nil, err
		}
	}

	return payment, nil
}

func (p *Payment) UncommittedEvents() []Event {
	return p.uncommittedEvents
}
