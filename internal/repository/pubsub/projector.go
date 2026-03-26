package pubsub

import (
	"context"
	"encoding/json"
	"log/slog"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type PaymentProjector struct {
	projection port.PaymentProjection
	reader     *kafka.Reader
}

func NewPaymentProjector(projection port.PaymentProjection, reader *kafka.Reader) *PaymentProjector {
	return &PaymentProjector{projection: projection, reader: reader}
}

func (p *PaymentProjector) Start(ctx context.Context) error {
	slog.Info("payment projector started")

	for {
		msg, err := p.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				slog.Info("payment projector stopped")
				return nil
			}

			slog.Error("payment projector: error reading", "error", err)
			continue
		}

		var event payment.Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("payment projector: error unmarshalling", "error", err)
			continue
		}

		_, span := tracer.Start(ctx, "PaymentProjector.ProjectEvent")
		span.SetAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.source", "payments.events"),
			attribute.String("aggregate.id", event.AggregateID),
			attribute.String("event.type", string(event.Type)),
		)

		if err := p.projection.Project(ctx, event.AggregateID); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.Error("payment projector: error projecting", "aggregateID", event.AggregateID, "error", err)
		}

		span.End()
	}
}
