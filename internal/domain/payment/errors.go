package payment

import "errors"

var (
	ErrInvalidAmount     = errors.New("Invalid amount")
	ErrInvalidCurrency   = errors.New("Invalid currency")
	ErrInvalidTransition = errors.New("Cannot transition payment")
	ErrUnknownEventType  = errors.New("Unknown event type")
	ErrNoEvents          = errors.New("No events provided")
	ErrPaymentFailed       = errors.New("Payment authorization failed")
	ErrConcurrencyConflict = errors.New("Concurrency conflict")
)
