// Package filter builds parameterized predicates from trusted column names.
package filter

import (
	"fmt"
	db "github.com/vexxhost/openstackdb"
	"strings"
	"time"
)

type Builder struct {
	Clauses []string
	Args    []any
}

// Strings treats nil as unrestricted and a non-nil empty slice as matching none.
func (b *Builder) Strings(column string, values []string) {
	if values == nil {
		return
	}
	if len(values) == 0 {
		b.Clauses = append(b.Clauses, "1 = 0")
		return
	}
	b.Clauses = append(b.Clauses, column+" IN ("+strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")+")")
	for _, v := range values {
		b.Args = append(b.Args, v)
	}
}

func (b *Builder) Deleted(column string, policy db.Deleted) error {
	switch policy {
	case db.ExcludeDeleted:
		b.Clauses = append(b.Clauses, column+" = 0")
	case db.IncludeDeleted:
	case db.OnlyDeleted:
		b.Clauses = append(b.Clauses, column+" <> 0")
	default:
		return fmt.Errorf("invalid deletion policy: %d", policy)
	}
	return nil
}

func (b *Builder) Lifetime(created, deleted string, start, end time.Time) error {
	if start.IsZero() && end.IsZero() {
		return nil
	}
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return fmt.Errorf("lifetime window requires start before end")
	}
	b.Clauses = append(b.Clauses, created+" < ?", "("+deleted+" IS NULL OR "+deleted+" > ?)")
	b.Args = append(b.Args, end.UTC(), start.UTC())
	return nil
}

func (b *Builder) SQL(base string) (string, []any) {
	if len(b.Clauses) != 0 {
		base += " WHERE " + strings.Join(b.Clauses, " AND ")
	}
	return base, b.Args
}
