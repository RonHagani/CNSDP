// Package migrations embeds the SQL migration files so the application
// binary carries them without depending on a filesystem path at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
