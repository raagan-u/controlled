// Package migrations embeds Controlled's versioned PostgreSQL schema migrations.
package migrations

import "embed"

// Files contains every checked-in up and down migration.
//
//go:embed *.sql
var Files embed.FS
