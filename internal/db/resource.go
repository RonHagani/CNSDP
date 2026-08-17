package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// SQLStateDiskFull is PostgreSQL's SQLSTATE for the disk_full member of
// class 53 "Insufficient Resources" -- the persistent-storage exhaustion
// condition NFR-036/AC-023 concern. Deliberately not the whole class:
// 53200 (out_of_memory) and 53300 (too_many_connections) are different
// resource classes and must not be classified this way. Confirmed
// empirically against the exact postgres:16-alpine image this repository
// uses (AC-023 empirical spike, 2026-08-15): a bounded tablespace filled
// to capacity produced exactly this code, via the same routine
// (mdzeroextend), across heap, index, and TOAST relation writes alike --
// never a different code, never a PANIC or crash-restart.
const SQLStateDiskFull = "53100"

// IsResourceExhausted reports whether err is a PostgreSQL error with
// SQLSTATE 53100 (disk_full) -- matched by code only, via pgconn.PgError,
// never by message-text matching. err may be wrapped arbitrarily deep
// (errors.As walks the chain), so this is safe to call directly on an
// error returned by internal/submission or any workflow-stage module's
// Advance function without those callers needing to know about pgconn
// themselves.
func IsResourceExhausted(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == SQLStateDiskFull
}
