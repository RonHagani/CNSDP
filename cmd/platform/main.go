// Command platform is the single deployable for the Cloud-Native Security
// Telemetry and Detection Platform (ARCH-01). At this checkpoint it
// connects to PostgreSQL, applies migrations, loads the version-controlled
// detection definitions (ADR-0004), and serves the telemetry admission
// endpoint (module 1) and the alert retrieval endpoint (module 8, ARCH-01
// §2) — the worker's processing loop is not started here yet.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"cnsdp/internal/db"
	"cnsdp/internal/detection"
	"cnsdp/internal/intake"
	"cnsdp/internal/retrieval"
)

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

	ctx := context.Background()

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
	log.Printf("listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
