package payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePayment_Success(t *testing.T) {
	p, err := CreatePayment("key-1", 1000, USD)

	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "key-1", p.IdempotencyKey)
	assert.Equal(t, int64(1000), p.Amount)
	assert.Equal(t, USD, p.Currency)
	assert.Equal(t, New, p.Status)
	assert.Equal(t, 1, p.Version)
	assert.Len(t, p.UncommittedEvents(), 1)
	assert.Equal(t, PaymentCreated, p.UncommittedEvents()[0].Type)
}

func TestCreatePayment_InvalidAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
	}{
		{"zero", 0},
		{"negative", -100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CreatePayment("key", tc.amount, USD)
			assert.ErrorIs(t, err, ErrInvalidAmount)
		})
	}
}

func TestCreatePayment_InvalidCurrency(t *testing.T) {
	_, err := CreatePayment("key", 1000, Currency("INVALID"))
	assert.ErrorIs(t, err, ErrInvalidCurrency)
}

func TestStartProcessing_FromNew(t *testing.T) {
	p, _ := CreatePayment("key", 1000, USD)
	initialVersion := p.Version

	err := p.StartProcessing()

	require.NoError(t, err)
	assert.Equal(t, Processing, p.Status)
	assert.Equal(t, initialVersion+1, p.Version)
	assert.Equal(t, ProcessingStarted, p.UncommittedEvents()[len(p.UncommittedEvents())-1].Type)
}

func TestStartProcessing_InvalidTransition(t *testing.T) {
	p, _ := CreatePayment("key", 1000, USD)
	p.StartProcessing()
	p.Complete()

	err := p.StartProcessing()
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestComplete_Success(t *testing.T) {
	p, _ := CreatePayment("key", 1000, USD)
	p.StartProcessing()

	err := p.Complete()

	require.NoError(t, err)
	assert.Equal(t, Completed, p.Status)
}

func TestComplete_InvalidTransition(t *testing.T) {
	p, _ := CreatePayment("key", 1000, USD)
	
	err := p.Complete()
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestFail_Success(t *testing.T) {
	p, _ := CreatePayment("key", 1000, USD)
	p.StartProcessing()

	err := p.Fail()

	require.NoError(t, err)
	assert.Equal(t, Failed, p.Status)
}

func TestCancel_FromNew(t *testing.T) {
	p, _ := CreatePayment("key", 1000, USD)

	err := p.Cancel()

	require.NoError(t, err)
	assert.Equal(t, Canceled, p.Status)
}

func TestCancel_FromProcessing(t *testing.T) {
	p, _ := CreatePayment("key", 1000, USD)
	p.StartProcessing()

	err := p.Cancel()

	require.NoError(t, err)
	assert.Equal(t, Canceled, p.Status)
}

func TestCancel_InvalidTransition(t *testing.T) {
	p, _ := CreatePayment("key", 1000, USD)
	p.StartProcessing()
	p.Complete()

	err := p.Cancel()
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestReconstructFromEvents_Success(t *testing.T) {
	original, _ := CreatePayment("key", 500, RUB)
	original.StartProcessing()
	original.Complete()

	events := original.UncommittedEvents()
	for i := range events {
		events[i].AggregateID = original.ID
	}

	reconstructed, err := ReconstructFromEvents(events)
	require.NoError(t, err)
	assert.Equal(t, Completed, reconstructed.Status)
	assert.Equal(t, original.Version, reconstructed.Version)
}

func TestReconstructFromEvents_EmptyList(t *testing.T) {
	_, err := ReconstructFromEvents([]Event{})
	assert.ErrorIs(t, err, ErrNoEvents)
}

func TestApplyEvent_UnknownType(t *testing.T) {
	p := &Payment{}
	err := p.ApplyEvent(Event{Type: PaymentEvent("unknown_event")})
	assert.ErrorIs(t, err, ErrUnknownEventType)
}
