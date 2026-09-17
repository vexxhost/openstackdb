// Package schema contains minimal database fixtures for integration tests.
// These fixtures are not migrations and must not be applied to a production database.
package schema

import "embed"

// Files contains service schemas, indexes, and prerequisites used by tests.
//
//go:embed */*.sql
var Files embed.FS
