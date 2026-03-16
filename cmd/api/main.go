package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"payment-gateway/internal/adapter/postgres"
	"payment-gateway/internal/adapter/provider"
	"payment-gateway/internal/adapter/pubsub"
	"payment-gateway/internal/app"
	"payment-gateway/internal/config"
	transport "payment-gateway/internal/transport/http"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	shutdownPeriod := 25 * time.Second
	shutdownHardPeriod := 5 * time.Second

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	pool, err := pgxpool.New(rootCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalln("Failed to create pool:", err)
	}
	defer pool.Close()

	eventStore := postgres.NewEventStore(pool)
	readRepo := postgres.NewPaymentReadRepository(pool)
	paymentProvider := provider.NewPaymentProvider()
	publisher := pubsub.NewEventPublisher(cfg.KafkaBrokers)

	paymentService := app.NewPaymentService(eventStore, readRepo, paymentProvider, publisher)

	paymentHandler := transport.NewPaymentHandler(paymentService)
	router := transport.NewRouter(paymentHandler)

	ongoingCtx, stopOngoing := context.WithCancel(context.Background())
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
		BaseContext: func(_ net.Listener) context.Context {
			return ongoingCtx
		},
	}

	go func() {
		log.Println("Starting server on", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln("Failed to start server:", err)
		}
	}()

	<-rootCtx.Done()
	stop()

	log.Println("Shutting down server...")
	stopOngoing()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownPeriod)
	defer cancel()

	shutdownErr := server.Shutdown(shutdownCtx)

	if shutdownErr != nil {
		log.Println("Server shutdown error:", shutdownErr)
		time.Sleep(shutdownHardPeriod)
		os.Exit(1)
	}

	log.Println("Server shutdown successfully")
}
