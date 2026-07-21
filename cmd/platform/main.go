// Command platform is the single deployable for the Cloud-Native Security
// Telemetry and Detection Platform (ARCH-01). At this checkpoint it only
// connects to PostgreSQL and applies migrations — intake, the worker, and
// every workflow-stage module are not implemented yet.
package main

import (
	"context"
	"log"
	"os"

	"cnsdp/internal/db"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
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

	log.Println("database connected and migrations applied")
}
