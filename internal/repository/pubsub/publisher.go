package pubsub

import (
	"context"
	"encoding/json"
	"payment-gateway/internal/domain/payment"

	"github.com/segmentio/kafka-go"
)

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
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.eventWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.AggregateID),
		Value: data,
	})
}

func (p *EventPublisher) PublishCommand(ctx context.Context, cmd string) error {
	return p.commandWriter.WriteMessages(ctx, kafka.Message{
		Value: []byte(cmd),
	})
}
