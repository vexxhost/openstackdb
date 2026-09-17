package filter

import (
	"github.com/stretchr/testify/require"
	db "github.com/vexxhost/openstackdb"
	"testing"
	"time"
)

func TestCombinedFilters(t *testing.T) {
	var b Builder
	require.NoError(t, b.Deleted("deleted", db.ExcludeDeleted))
	b.Strings("project_id", []string{"a", "b' OR 1=1 --"})
	b.Strings("uuid", []string{})
	query, args := b.SQL("SELECT uuid FROM instances")
	require.Equal(t, "SELECT uuid FROM instances WHERE deleted = 0 AND project_id IN (?,?) AND 1 = 0", query)
	require.Equal(t, []any{"a", "b' OR 1=1 --"}, args)
}
func TestNilAndDeleted(t *testing.T) {
	var b Builder
	b.Strings("uuid", nil)
	require.NoError(t, b.Deleted("deleted", db.IncludeDeleted))
	q, args := b.SQL("SELECT uuid FROM instances")
	require.Equal(t, "SELECT uuid FROM instances", q)
	require.Empty(t, args)
	require.NoError(t, b.Deleted("deleted", db.OnlyDeleted))
	q, _ = b.SQL("SELECT uuid FROM instances")
	require.Contains(t, q, "deleted <> 0")
	require.Error(t, b.Deleted("deleted", db.Deleted(99)))
}
func TestLifetime(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	var b Builder
	require.NoError(t, b.Lifetime("created_at", "deleted_at", start, end))
	q, args := b.SQL("SELECT uuid FROM instances")
	require.Equal(t, "SELECT uuid FROM instances WHERE created_at < ? AND (deleted_at IS NULL OR deleted_at > ?)", q)
	require.Equal(t, []any{end, start}, args)
	require.Error(t, b.Lifetime("created_at", "deleted_at", end, start))
	require.Error(t, b.Lifetime("created_at", "deleted_at", start, time.Time{}))
}
