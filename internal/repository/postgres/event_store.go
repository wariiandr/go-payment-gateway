package postgres

import (
	"context"
	"errors"
	"payment-gateway/internal/domain/payment"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("payment-gateway/repository/postgres")

type EventStore struct {
	pool *pgxpool.Pool
}

func NewEventStore(pool *pgxpool.Pool) *EventStore {
	return &EventStore{pool: pool}
}

func (store *EventStore) SaveEvents(ctx context.Context, aggregateId string, events []payment.Event, expectedVersion int) error {
	ctx, span := tracer.Start(ctx, "EventStore.SaveEvents")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("aggregate.id", aggregateId),
		attribute.Int("events.count", len(events)),
	)

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO payment_events (aggregate_id, event_type, version, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	for _, event := range events {
		_, err = tx.Exec(ctx, query,
			aggregateId, event.Type, event.Version, event.Payload, event.CreatedAt,
		)

		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				span.RecordError(payment.ErrConcurrencyConflict)
				span.SetStatus(codes.Error, "concurrency conflict")
				return payment.ErrConcurrencyConflict
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func (store *EventStore) LoadEvents(ctx context.Context, aggregateId string) ([]payment.Event, error) {
	ctx, span := tracer.Start(ctx, "EventStore.LoadEvents")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("aggregate.id", aggregateId),
	)

	query := `
		SELECT id, aggregate_id, event_type, version, payload, created_at
		FROM payment_events
		WHERE aggregate_id = $1
		ORDER BY version ASC`

	rows, err := store.pool.Query(ctx, query, aggregateId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	defer rows.Close()

	var events []payment.Event

	for rows.Next() {
		var event payment.Event
		err := rows.Scan(
			&event.ID, &event.AggregateID, &event.Type,
			&event.Version, &event.Payload, &event.CreatedAt,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.Int("events.loaded", len(events)))
	return events, nil
}
