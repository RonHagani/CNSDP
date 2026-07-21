// Command platform is the single deployable for the Cloud-Native Security
// Telemetry and Detection Platform (ARCH-01). At this checkpoint it
// connects to PostgreSQL, applies migrations, and loads the
// version-controlled detection definitions (ADR-0004) — intake, the
// worker's stage handlers beyond validate, and every other
// workflow-stage module are not implemented yet.
package main

import (
	"context"
	"log"
	"os"

	"cnsdp/internal/db"
	"cnsdp/internal/detection"
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

	if err := detection.Load(ctx, conn); err != nil {
		log.Fatalf("load detection definitions: %v", err)
	}

	log.Println("database connected, migrations applied, detection definitions loaded")
}
