package service

import (
	"context"
	"errors"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"
	"payment-gateway/internal/service/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(
	eventStore port.EventStore,
	readRepo port.PaymentReadRepository,
	provider port.PaymentProvider,
	publisher port.PaymentPublisher,
	commandRepo port.CommandRepository,
) *PaymentService {
	return NewPaymentService(eventStore, readRepo, provider, publisher, commandRepo)
}

func buildEventListForPayment(key string, amount int64, currency payment.Currency) []payment.Event {
	p, _ := payment.CreatePayment(key, amount, currency)
	return p.UncommittedEvents()
}

func TestService_CreatePayment_Success(t *testing.T) {
	ctx := context.Background()

	publishedCmds := 0
	savedEvents := 0

	svc := newTestService(
		&mocks.MockEventStore{
			SaveEventsFn: func(_ context.Context, _ string, _ []payment.Event, _ int) error {
				savedEvents++
				return nil
			},
		},
		&mocks.MockReadRepository{},
		&mocks.MockPaymentProvider{},
		&mocks.MockPublisher{
			PublishCommandFn: func(_ context.Context, _ string) error {
				publishedCmds++
				return nil
			},
		},
		&mocks.MockCommandRepository{},
	)

	id, err := svc.CreatePayment(ctx, &CreatePaymentRequest{
		IdempotencyKey: "key-1",
		Amount:         1000,
		Currency:       payment.USD,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Equal(t, 1, savedEvents)
	assert.Equal(t, 1, publishedCmds)
}

func TestService_CreatePayment_InvalidAmount(t *testing.T) {
	ctx := context.Background()
	savedEvents := 0

	svc := newTestService(
		&mocks.MockEventStore{
			SaveEventsFn: func(_ context.Context, _ string, _ []payment.Event, _ int) error {
				savedEvents++
				return nil
			},
		},
		&mocks.MockReadRepository{},
		&mocks.MockPaymentProvider{},
		&mocks.MockPublisher{},
		&mocks.MockCommandRepository{},
	)

	_, err := svc.CreatePayment(ctx, &CreatePaymentRequest{
		IdempotencyKey: "key",
		Amount:         0,
		Currency:       payment.USD,
	})

	assert.ErrorIs(t, err, payment.ErrInvalidAmount)
	assert.Equal(t, 0, savedEvents)
}

func TestService_CreatePayment_SaveEventsFails_NoPublish(t *testing.T) {
	ctx := context.Background()
	publishedCmds := 0
	saveErr := errors.New("db error")

	svc := newTestService(
		&mocks.MockEventStore{
			SaveEventsFn: func(_ context.Context, _ string, _ []payment.Event, _ int) error {
				return saveErr
			},
		},
		&mocks.MockReadRepository{},
		&mocks.MockPaymentProvider{},
		&mocks.MockPublisher{
			PublishCommandFn: func(_ context.Context, _ string) error {
				publishedCmds++
				return nil
			},
		},
		&mocks.MockCommandRepository{},
	)

	_, err := svc.CreatePayment(ctx, &CreatePaymentRequest{
		IdempotencyKey: "key",
		Amount:         1000,
		Currency:       payment.USD,
	})

	assert.ErrorIs(t, err, saveErr)
	assert.Equal(t, 0, publishedCmds)
}

func TestService_ProcessPayment_Duplicate(t *testing.T) {
	ctx := context.Background()
	loadEventsCalled := false

	svc := newTestService(
		&mocks.MockEventStore{
			LoadEventsFn: func(_ context.Context, _ string) ([]payment.Event, error) {
				loadEventsCalled = true
				return nil, nil
			},
		},
		&mocks.MockReadRepository{},
		&mocks.MockPaymentProvider{},
		&mocks.MockPublisher{},
		&mocks.MockCommandRepository{
			IsProcessedFn: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
		},
	)

	err := svc.ProcessPayment(ctx, "cmd-id", "pay-id")

	require.NoError(t, err)
	assert.False(t, loadEventsCalled)
}

func TestService_ProcessPayment_AuthorizeSuccess(t *testing.T) {
	ctx := context.Background()

	events := buildEventListForPayment("k", 1000, payment.USD)
	markedResult := port.CommandResult("")

	svc := newTestService(
		&mocks.MockEventStore{
			LoadEventsFn: func(_ context.Context, _ string) ([]payment.Event, error) {
				return events, nil
			},
			SaveEventsFn: func(_ context.Context, _ string, _ []payment.Event, _ int) error {
				return nil
			},
		},
		&mocks.MockReadRepository{},
		&mocks.MockPaymentProvider{
			AuthorizeFn: func(_ context.Context, _ payment.Payment) error {
				return nil
			},
		},
		&mocks.MockPublisher{},
		&mocks.MockCommandRepository{
			IsProcessedFn: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			MarkProcessedFn: func(_ context.Context, _ string, result port.CommandResult) error {
				markedResult = result
				return nil
			},
		},
	)

	err := svc.ProcessPayment(ctx, "cmd-id", "pay-id")

	require.NoError(t, err)
	assert.Equal(t, port.CommandResultCompleted, markedResult)
}

func TestService_ProcessPayment_AuthorizeFails(t *testing.T) {
	ctx := context.Background()

	events := buildEventListForPayment("k", 1000, payment.USD)
	markedResult := port.CommandResult("")
	authErr := errors.New("psp error")

	svc := newTestService(
		&mocks.MockEventStore{
			LoadEventsFn: func(_ context.Context, _ string) ([]payment.Event, error) {
				return events, nil
			},
			SaveEventsFn: func(_ context.Context, _ string, _ []payment.Event, _ int) error {
				return nil
			},
		},
		&mocks.MockReadRepository{},
		&mocks.MockPaymentProvider{
			AuthorizeFn: func(_ context.Context, _ payment.Payment) error {
				return authErr
			},
		},
		&mocks.MockPublisher{},
		&mocks.MockCommandRepository{
			IsProcessedFn: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			MarkProcessedFn: func(_ context.Context, _ string, result port.CommandResult) error {
				markedResult = result
				return nil
			},
		},
	)

	err := svc.ProcessPayment(ctx, "cmd-id", "pay-id")

	require.NoError(t, err)
	assert.Equal(t, port.CommandResultFailed, markedResult)
}

func TestService_GetPayment_Success(t *testing.T) {
	ctx := context.Background()
	want := &payment.Payment{ID: "pay-1", Status: payment.Completed}

	svc := newTestService(
		&mocks.MockEventStore{},
		&mocks.MockReadRepository{
			GetPaymentByIdFn: func(_ context.Context, id string) (*payment.Payment, error) {
				return want, nil
			},
		},
		&mocks.MockPaymentProvider{},
		&mocks.MockPublisher{},
		&mocks.MockCommandRepository{},
	)

	got, err := svc.GetPayment(ctx, "pay-1")
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
}

func TestService_GetPayment_NotFound(t *testing.T) {
	ctx := context.Background()
	notFoundErr := errors.New("not found")

	svc := newTestService(
		&mocks.MockEventStore{},
		&mocks.MockReadRepository{
			GetPaymentByIdFn: func(_ context.Context, _ string) (*payment.Payment, error) {
				return nil, notFoundErr
			},
		},
		&mocks.MockPaymentProvider{},
		&mocks.MockPublisher{},
		&mocks.MockCommandRepository{},
	)

	_, err := svc.GetPayment(ctx, "pay-1")
	assert.ErrorIs(t, err, notFoundErr)
}

func TestService_CancelPayment_Success(t *testing.T) {
	ctx := context.Background()
	events := buildEventListForPayment("k", 500, payment.RUB)

	publishedEvents := 0

	svc := newTestService(
		&mocks.MockEventStore{
			LoadEventsFn: func(_ context.Context, _ string) ([]payment.Event, error) {
				return events, nil
			},
			SaveEventsFn: func(_ context.Context, _ string, _ []payment.Event, _ int) error {
				return nil
			},
		},
		&mocks.MockReadRepository{},
		&mocks.MockPaymentProvider{},
		&mocks.MockPublisher{
			PublishEventFn: func(_ context.Context, _ payment.Event) error {
				publishedEvents++
				return nil
			},
		},
		&mocks.MockCommandRepository{},
	)

	err := svc.CancelPayment(ctx, "pay-id")
	require.NoError(t, err)
	assert.Greater(t, publishedEvents, 0)
}

func TestService_CancelPayment_InvalidTransition(t *testing.T) {
	ctx := context.Background()

	p, _ := payment.CreatePayment("k", 500, payment.RUB)
	p.StartProcessing()
	p.Complete()
	events := p.UncommittedEvents()

	svc := newTestService(
		&mocks.MockEventStore{
			LoadEventsFn: func(_ context.Context, _ string) ([]payment.Event, error) {
				return events, nil
			},
		},
		&mocks.MockReadRepository{},
		&mocks.MockPaymentProvider{},
		&mocks.MockPublisher{},
		&mocks.MockCommandRepository{},
	)

	err := svc.CancelPayment(ctx, "pay-id")
	assert.ErrorIs(t, err, payment.ErrInvalidTransition)
}
