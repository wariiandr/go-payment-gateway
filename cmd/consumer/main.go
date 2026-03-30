package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"payment-gateway/internal/config"
	"payment-gateway/internal/infra/telemetry"
	"payment-gateway/internal/repository/postgres"
	"payment-gateway/internal/repository/provider"
	"payment-gateway/internal/repository/pubsub"
	"payment-gateway/internal/service"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	shutdownTracer, err := telemetry.InitTracer(rootCtx, "payment-gateway-consumer", cfg.OTelEndpoint)
	if err != nil {
		slog.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdownTracer(rootCtx)

	shutdownMetrics, metricsHandler, err := telemetry.InitMetrics()
	if err != nil {
		slog.Error("failed to init metrics", "error", err)
		os.Exit(1)
	}
	defer shutdownMetrics(rootCtx)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metricsHandler)

		slog.Info("starting metrics server for consumer", "addr", ":8082")

		if err := http.ListenAndServe(":8082", mux); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	pool, err := pgxpool.New(rootCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	var conn *kafka.Conn
	for i := 0; i < 30; i++ {
		conn, err = kafka.Dial("tcp", cfg.KafkaBrokers[0])
		if err == nil {
			break
		}
		slog.Warn("waiting for Kafka", "attempt", i+1, "error", err)
		time.Sleep(1 * time.Second)
	}
	if conn == nil {
		slog.Error("failed to connect to Kafka after 30 attempts")
		os.Exit(1)
	}

	topicConfigs := []kafka.TopicConfig{
		{Topic: "payments.commands", NumPartitions: 1, ReplicationFactor: 1},
		{Topic: "payments.events", NumPartitions: 1, ReplicationFactor: 1},
	}
	err = conn.CreateTopics(topicConfigs...)
	if err != nil {
		slog.Warn("create topics (may already exist)", "error", err)
	} else {
		slog.Info("topics created successfully")
	}
	conn.Close()

	commandReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.KafkaBrokers,
		Topic:       "payments.commands",
		GroupID:     "payment-worker",
		StartOffset: kafka.FirstOffset,
		Logger:      log.New(os.Stdout, "kafka-cmd: ", 0),
		ErrorLogger: log.New(os.Stderr, "kafka-cmd-ERR: ", 0),
	})
	defer commandReader.Close()

	eventStore := postgres.NewEventStore(pool)
	readRepo := postgres.NewPaymentReadRepository(pool)
	commandRepo := postgres.NewCommandRepository(pool)
	paymentProvider := provider.NewPaymentProvider()
	publisher := pubsub.NewEventPublisher(cfg.KafkaBrokers)

	paymentService := service.NewPaymentService(eventStore, readRepo, paymentProvider, publisher, commandRepo)
	worker := pubsub.NewPaymentWorker(commandReader, paymentService)

	go worker.Start(rootCtx)

	eventReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.KafkaBrokers,
		Topic:       "payments.events",
		GroupID:     "payment-projector",
		StartOffset: kafka.FirstOffset,
		Logger:      log.New(os.Stdout, "kafka-evt: ", 0),
		ErrorLogger: log.New(os.Stderr, "kafka-evt-ERR: ", 0),
	})
	defer eventReader.Close()

	projector := pubsub.NewPaymentProjector(paymentService, eventReader)
	go projector.Start(rootCtx)

	slog.Info("consumer started")
	<-rootCtx.Done()
	stop()

	slog.Info("shutting down consumer")
	slog.Info("consumer stopped")
}
