package app

import (
	"context"
	"encoding/json"
	"log"
	"payment-gateway/internal/domain/payment"
	"payment-gateway/internal/port"

	"github.com/segmentio/kafka-go"
)

type PaymentProjector struct {
	readRepo   port.PaymentReadRepository
	eventStore port.EventStore
	reader     *kafka.Reader
}

func NewPaymentProjector(readRepo port.PaymentReadRepository, eventStore port.EventStore, reader *kafka.Reader) *PaymentProjector {
	return &PaymentProjector{readRepo: readRepo, eventStore: eventStore, reader: reader}
}

func (p *PaymentProjector) Start(ctx context.Context) error {
	log.Println("Payment projector started")

	for {
		msg, err := p.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Payment projector stopped")
				return nil
			}

			log.Printf("Payment projector: error reading: %v", err)
			continue
		}

		var event payment.Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("Payment projector: error unmarshalling: %v", err)
			continue
		}

		if err := p.Project(ctx, event); err != nil {
			log.Printf("Payment projector: error projecting: %v", err)
			continue
		}
	}
}

func (p *PaymentProjector) Project(ctx context.Context, event payment.Event) error {
	events, err := p.eventStore.LoadEvents(ctx, event.AggregateID)
	if err != nil {
		return err
	}

	paym, err := payment.ReconstructFromEvents(events)
	if err != nil {
		return err
	}

	return p.readRepo.Upsert(ctx, paym)
}
