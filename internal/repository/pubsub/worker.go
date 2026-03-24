package pubsub

import (
	"context"
	"log/slog"
	"payment-gateway/internal/port"
	"strings"

	"github.com/segmentio/kafka-go"
)

type PaymentWorker struct {
	reader    *kafka.Reader
	processor port.PaymentProcessor
}

func NewPaymentWorker(reader *kafka.Reader, processor port.PaymentProcessor) *PaymentWorker {
	return &PaymentWorker{reader: reader, processor: processor}
}

func (w *PaymentWorker) Start(ctx context.Context) error {
	slog.Info("payment worker started")

	for {
		msg, err := w.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				slog.Info("payment worker: context cancelled")
				return nil
			}
			slog.Error("payment worker: error reading message", "error", err)
			continue
		}

		parts := strings.SplitN(string(msg.Value), ":", 2)
		if len(parts) != 2 || parts[0] != "process_payment" {
			slog.Warn("payment worker: invalid message format", "value", string(msg.Value))
			continue
		}

		paymentID := parts[1]

		err = w.processor.ProcessPayment(ctx, paymentID)
		if err != nil {
			slog.Error("payment worker: error processing payment", "paymentID", paymentID, "error", err)
			continue
		}
	}
}
