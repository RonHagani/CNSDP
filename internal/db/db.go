// Package db owns the platform's PostgreSQL connection and schema
// migrations. It has no knowledge of any workflow-stage module's artifact
// tables — it opens a connection pool, applies migrations, and classifies
// the one PostgreSQL-native error condition (resource.go) multiple
// callers need to distinguish from an ordinary processing failure.
package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Connect opens a connection pool for dsn and verifies it is reachable.
func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return conn, nil
}
