//go:build integration

// Package testutil provides the ephemeral-PostgreSQL bootstrap shared by
// every integration test package (test/integration, internal/submission,
// internal/worker). It exists to avoid triplicating the same
// testcontainers setup across those three locations.
package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"cnsdp/internal/db"
)

// MigratedPostgres starts an ephemeral PostgreSQL container, connects to
// it, and applies every migration. The container and connection are
// cleaned up automatically when the test completes.
func MigratedPostgres(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("cnsdp_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("resolve connection string: %v", err)
	}

	conn, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := db.RunMigrations(conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return conn
}
