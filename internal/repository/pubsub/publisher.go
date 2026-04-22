package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("payment-gateway/repository/pubsub")

type EventPublisher struct {
	commandWriter *kafka.Writer
	eventWriter   *kafka.Writer
}

func NewEventPublisher(brokers []string) *EventPublisher {
	return &EventPublisher{
		commandWriter: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  "payments.commands",
			AllowAutoTopicCreation: true,
		},
		eventWriter: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Topic:                  "payments.events",
			AllowAutoTopicCreation: true,
		},
	}
}

func (p *EventPublisher) PublishEvent(ctx context.Context, event payment.Event) error {
	ctx, span := tracer.Start(ctx, "Publisher.PublishEvent")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", "payments.events"),
		attribute.String("event.type", string(event.Type)),
		attribute.String("aggregate.id", event.AggregateID),
	)

	data, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "publisher: marshal event failed",
			"event.type", event.Type,
			"aggregate.id", event.AggregateID,
			"error", err,
		)
		return err
	}

	err = p.eventWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.AggregateID),
		Value: data,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "publisher: write event failed",
			"topic", "payments.events",
			"event.type", event.Type,
			"aggregate.id", event.AggregateID,
			"error", err,
		)
		return errors.Join(port.ErrMessaging, err)
	}

	return nil
}

func (p *EventPublisher) PublishCommand(ctx context.Context, cmd string) error {
	ctx, span := tracer.Start(ctx, "Publisher.PublishCommand")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", "payments.commands"),
		attribute.String("command", cmd),
	)

	err := p.commandWriter.WriteMessages(ctx, kafka.Message{
		Value: []byte(cmd),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "publisher: write command failed",
			"topic", "payments.commands",
			"command", cmd,
			"error", err,
		)
		return errors.Join(port.ErrMessaging, err)
	}

	return nil
}
