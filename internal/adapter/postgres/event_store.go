package postgres

import (
	"context"
	"errors"
	"payment-gateway/internal/domain/payment"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventStore struct {
	pool *pgxpool.Pool
}

func NewEventStore(pool *pgxpool.Pool) *EventStore {
	return &EventStore{pool: pool}
}

func (store *EventStore) SaveEvents(ctx context.Context, aggregateId string, events []payment.Event, expectedVersion int) error {
	tx, err := store.pool.Begin(ctx)

	if err != nil {
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
				return payment.ErrConcurrencyConflict
			}
			return err
		}
	}

	return tx.Commit(ctx)
}

func (store *EventStore) LoadEvents(ctx context.Context, aggregateId string) ([]payment.Event, error) {
	query := `
		SELECT id, aggregate_id, event_type, version, payload, created_at
		FROM payment_events
		WHERE aggregate_id = $1
		ORDER BY version ASC`

	rows, err := store.pool.Query(ctx, query, aggregateId)

	if err != nil {
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
			return nil, err
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}
