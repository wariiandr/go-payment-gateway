package postgres

import (
	"context"
	"payment-gateway/internal/port"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type CommandRepository struct {
	pool *pgxpool.Pool
}

func NewCommandRepository(pool *pgxpool.Pool) *CommandRepository {
	return &CommandRepository{pool: pool}
}

func (r *CommandRepository) IsProcessed(ctx context.Context, commandID string) (bool, error) {
	ctx, span := tracer.Start(ctx, "CommandRepository.IsProcessed")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("command.id", commandID),
	)

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM processed_commands WHERE command_id = $1)`
	err := r.pool.QueryRow(ctx, query, commandID).Scan(&exists)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	return exists, nil
}

func (r *CommandRepository) MarkProcessed(ctx context.Context, commandID string, result port.CommandResult) error {
	ctx, span := tracer.Start(ctx, "CommandRepository.MarkProcessed")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("command.id", commandID),
		attribute.String("command.result", string(result)),
	)

	query := `
		INSERT INTO processed_commands (command_id, result)
		VALUES ($1, $2)
		ON CONFLICT (command_id) DO NOTHING`

	_, err := r.pool.Exec(ctx, query, commandID, result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}
