// Package migrations embeds the control plane's SQL migrations (goose format).
package migrations

import "embed"

// FS holds every migration file. Files are named NNNNN_description.sql and never edited once released.
//
//go:embed *.sql
var FS embed.FS
