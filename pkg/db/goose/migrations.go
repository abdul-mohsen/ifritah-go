// Package goosemigrations embeds the goose migrations for the tenant database.
//
// Only the numbered files in this directory are applied by goose. The legacy SQL under
// pkg/db/schema and pkg/db/migrations still drives sqlc and the existing replay check until
// the deployment tooling moves to goose.
package goosemigrations

import "embed"

// BaselineVersion is the goose version of 00001_baseline.sql. Existing tenant databases are
// adopted at this version, and goose never rolls the baseline back.
const BaselineVersion int64 = 1

// FS holds the goose migration files.
//
//go:embed *.sql
var FS embed.FS
