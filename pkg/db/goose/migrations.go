// Package goosemigrations embeds the goose migrations for the tenant database.
//
// Only the numbered files in this directory are applied by goose. sqlc reads this directory for
// the tenant schema, and pkg/db/schema/car_part.sql stays a separate sqlc input.
package goosemigrations

import "embed"

// BaselineVersion is the goose version of 00001_baseline.sql. Goose never rolls
// the baseline back.
const BaselineVersion int64 = 1

// FS holds the goose migration files.
//
//go:embed *.sql
var FS embed.FS
