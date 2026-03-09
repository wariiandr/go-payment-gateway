package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func main() {
	shutdownPeriod := 25 * time.Second
	shutdownHardPeriod := 5 * time.Second

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello world %s", time.Now().Format(time.RFC3339))
	})

	ongoingCtx, stopOngoing := context.WithCancel(context.Background())
	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
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
