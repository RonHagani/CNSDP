// Command platform is the single deployable for the Cloud-Native Security
// Telemetry and Detection Platform (ARCH-01). It connects to PostgreSQL,
// applies migrations, loads the version-controlled detection definitions
// (ADR-0004), serves the telemetry admission endpoint (module 1) and the
// alert retrieval endpoint (module 8, ARCH-01 §2), and runs the durable
// worker's continuous processing loop (module 1-6 orchestration,
// internal/worker.Run) that drives admitted submissions through
// validated -> normalized -> evaluated -> alerted -> evidenced
// unattended -- completing ARCH-01 §8's walking skeleton end to end in
// this process, not only across separate package-level tests.
//
// Shutdown is signal-driven: SIGINT/SIGTERM cancel one shared context,
// which stops the worker loop and triggers a bounded-timeout HTTP
// graceful shutdown; the process waits for both to finish before exiting.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cnsdp/internal/db"
	"cnsdp/internal/detection"
	"cnsdp/internal/intake"
	"cnsdp/internal/retrieval"
	"cnsdp/internal/worker"
)

// shutdownTimeout bounds how long graceful HTTP shutdown waits for
// in-flight requests to finish once a shutdown signal is received.
const shutdownTimeout = 5 * time.Second

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	token := os.Getenv("API_TOKEN")
	if token == "" {
		log.Fatal("API_TOKEN is required")
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	maxBodyBytes := int64(intake.DefaultMaxBodyBytes)
	if v := os.Getenv("MAX_BODY_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			log.Fatalf("MAX_BODY_BYTES must be a positive integer, got %q", v)
		}
		maxBodyBytes = n
	}
	pollInterval := time.Second
	if v := os.Getenv("WORKER_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			log.Fatalf("WORKER_POLL_INTERVAL must be a positive duration, got %q", v)
		}
		pollInterval = d
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := db.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer conn.Close()

	if err := db.RunMigrations(conn); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	if err := detection.Load(ctx, conn); err != nil {
		log.Fatalf("load detection definitions: %v", err)
	}

	log.Println("database connected, migrations applied, detection definitions loaded")

	mux := http.NewServeMux()
	mux.Handle("POST /v1/audit-events", &intake.Handler{DB: conn, Token: token, MaxBodyBytes: maxBodyBytes})
	mux.Handle("GET /v1/alerts/{id}", &retrieval.Handler{DB: conn, Token: token})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		worker.Run(ctx, conn, pollInterval)
	}()

	var runErr error
	select {
	case <-ctx.Done():
		log.Println("shutdown signal received")
	case runErr = <-serverErr:
		if runErr != nil {
			log.Printf("http server error: %v", runErr)
			stop() // cancel the shared context so the worker loop also stops
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown: %v", err)
	}

	<-workerDone
	log.Println("shutdown complete")

	if runErr != nil {
		log.Fatalf("exiting after http server error: %v", runErr)
	}
}
