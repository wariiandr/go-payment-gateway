package app

import (
	"context"
	"log"
	"strings"

	"github.com/segmentio/kafka-go"
)

type PaymentWorker struct {
	reader  *kafka.Reader
	service *PaymentService
}

func NewPaymentWorker(reader *kafka.Reader, service *PaymentService) *PaymentWorker {
	return &PaymentWorker{reader: reader, service: service}
}

func (w *PaymentWorker) Start(ctx context.Context) error {
	log.Println("Payment worker started")

	for {
		msg, err := w.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Payment worker: context cancelled")
				return nil
			}
			log.Printf("Payment worker: error reading message: %v", err)
			continue
		}

		parts := strings.SplitN(string(msg.Value), ":", 2)
		if len(parts) != 2 || parts[0] != "process_payment" {
			log.Printf("Payment worker: invalid message format: %s", string(msg.Value))
			continue
		}

		paymentID := parts[1]

		err = w.service.ProcessPayment(ctx, paymentID)
		if err != nil {
			log.Printf("Payment worker: error processing payment %s: %v", paymentID, err)
			continue
		}
	}
}
