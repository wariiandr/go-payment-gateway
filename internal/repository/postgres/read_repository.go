package postgres

import (
	"context"
	"payment-gateway/internal/domain/payment"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentReadRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentReadRepository(pool *pgxpool.Pool) *PaymentReadRepository {
	return &PaymentReadRepository{pool: pool}
}

func (repo *PaymentReadRepository) GetPaymentById(ctx context.Context, id string) (*payment.Payment, error) {
	query := `
		SELECT id, idempotency_key, amount, currency, status, version, created_at, updated_at
		FROM payments
		WHERE id = $1`

	var p payment.Payment
	err := repo.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.IdempotencyKey, &p.Amount, &p.Currency,
		&p.Status, &p.Version, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (repo *PaymentReadRepository) Upsert(ctx context.Context, p *payment.Payment) error {
	query := `
		INSERT INTO payments (id, idempotency_key, amount, currency, status, version)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (idempotency_key) DO UPDATE SET
			amount = EXCLUDED.amount,
			currency = EXCLUDED.currency,
			status = EXCLUDED.status,
			version = EXCLUDED.version,
			updated_at = now()`

	_, err := repo.pool.Exec(ctx, query,
		p.ID, p.IdempotencyKey, p.Amount, p.Currency, p.Status, p.Version,
	)

	if err != nil {
		return err
	}

	return nil
}
