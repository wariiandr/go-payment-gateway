package pubsub

import (
	"context"
	"log/slog"
	"payment-gateway/internal/port"
	"strings"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

		parts := strings.SplitN(string(msg.Value), ":", 3)
		if len(parts) != 3 || parts[0] != "process_payment" {
			slog.Warn("payment worker: invalid message format", "value", string(msg.Value))
			continue
		}

		commandID := parts[1]
		paymentID := parts[2]

		_, span := tracer.Start(ctx, "PaymentWorker.ProcessMessage")
		span.SetAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.source", "payments.commands"),
			attribute.String("command.id", commandID),
			attribute.String("payment.id", paymentID),
		)

		err = w.processor.ProcessPayment(ctx, commandID, paymentID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.Error("payment worker: error processing payment", "paymentID", paymentID, "error", err)
		}

		span.End()
	}
}
