// Package migrations embeds the SQL migration files so they can be applied at
// startup (and in tests) without shipping the .sql files alongside the binary.
package migrations

import "embed"

// FS holds every *.sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS
