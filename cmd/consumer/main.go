package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"payment-gateway/internal/adapter/postgres"
	"payment-gateway/internal/adapter/provider"
	"payment-gateway/internal/adapter/pubsub"
	"payment-gateway/internal/app"
	"payment-gateway/internal/config"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	pool, err := pgxpool.New(rootCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalln("Failed to create pool:", err)
	}
	defer pool.Close()

	var conn *kafka.Conn
	for i := 0; i < 30; i++ {
		conn, err = kafka.Dial("tcp", cfg.KafkaBrokers[0])
		if err == nil {
			break
		}
		log.Printf("Waiting for Kafka... (%d/30): %v", i+1, err)
		time.Sleep(1 * time.Second)
	}
	if conn == nil {
		log.Fatalln("Failed to connect to Kafka after 30 attempts")
	}

	topicConfigs := []kafka.TopicConfig{
		{Topic: "payments.commands", NumPartitions: 1, ReplicationFactor: 1},
		{Topic: "payments.events", NumPartitions: 1, ReplicationFactor: 1},
	}
	err = conn.CreateTopics(topicConfigs...)
	if err != nil {
		log.Println("CreateTopics (may already exist):", err)
	} else {
		log.Println("Topics created successfully")
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
	paymentProvider := provider.NewPaymentProvider()
	publisher := pubsub.NewEventPublisher(cfg.KafkaBrokers)

	paymentService := app.NewPaymentService(eventStore, readRepo, paymentProvider, publisher)
	worker := app.NewPaymentWorker(commandReader, paymentService)

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

	projector := app.NewPaymentProjector(readRepo, eventStore, eventReader)
	go projector.Start(rootCtx)

	log.Println("Consumer started")
	<-rootCtx.Done()
	stop()

	log.Println("Shutting down consumer...")

	log.Println("Consumer stopped")
}
