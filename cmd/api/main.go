package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"payment-gateway/internal/config"
	"payment-gateway/internal/infra/telemetry"
	"payment-gateway/internal/repository/postgres"
	"payment-gateway/internal/repository/provider"
	"payment-gateway/internal/repository/pubsub"
	"payment-gateway/internal/service"
	transport "payment-gateway/internal/transport/http"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	shutdownPeriod := 25 * time.Second
	shutdownHardPeriod := 5 * time.Second

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	shutdownTracer, err := telemetry.InitTracer(rootCtx, "payment-gateway-api", cfg.OTelEndpoint)
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

	pool, err := pgxpool.New(rootCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	eventStore := postgres.NewEventStore(pool)
	readRepo := postgres.NewPaymentReadRepository(pool)
	commandRepo := postgres.NewCommandRepository(pool)
	paymentProvider := provider.NewPaymentProvider()
	publisher := pubsub.NewEventPublisher(cfg.KafkaBrokers)

	paymentService := service.NewPaymentService(eventStore, readRepo, paymentProvider, publisher, commandRepo)

	paymentHandler := transport.NewPaymentHandler(paymentService)
	router := transport.NewRouter(paymentHandler, metricsHandler)

	ongoingCtx, stopOngoing := context.WithCancel(context.Background())
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
		BaseContext: func(_ net.Listener) context.Context {
			return ongoingCtx
		},
	}

	go func() {
		slog.Info("starting server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	<-rootCtx.Done()
	stop()

	slog.Info("shutting down server")
	stopOngoing()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownPeriod)
	defer cancel()

	shutdownErr := server.Shutdown(shutdownCtx)

	if shutdownErr != nil {
		slog.Error("server shutdown error", "error", shutdownErr)
		time.Sleep(shutdownHardPeriod)
		os.Exit(1)
	}

	slog.Info("server shutdown successfully")
}
