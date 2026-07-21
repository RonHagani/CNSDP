// Package definitions embeds the version-controlled detection-definition
// YAML files (ADR-0004) so the application binary carries them without
// depending on a filesystem path at runtime, mirroring the migrations
// package's embedding pattern.
package definitions

import "embed"

//go:embed *.yaml
var FS embed.FS
