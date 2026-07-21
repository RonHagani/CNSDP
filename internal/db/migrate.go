package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"cnsdp/migrations"
)

// RunMigrations applies every pending migration in migrations.FS against
// conn, using conn's own connection pool rather than opening a second one.
func RunMigrations(conn *sql.DB) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load embedded migration source: %w", err)
	}

	target, err := pgxmigrate.WithInstance(conn, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("attach migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx", target)
	if err != nil {
		return fmt.Errorf("construct migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
