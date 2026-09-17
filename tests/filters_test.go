//go:build integration

package tests

import (
	"context"
	"database/sql"
	"github.com/stretchr/testify/require"
	db "github.com/vexxhost/openstackdb"
	cinder "github.com/vexxhost/openstackdb/cinder/db"
	glance "github.com/vexxhost/openstackdb/glance/db"
	"github.com/vexxhost/openstackdb/internal/testutil"
	nova "github.com/vexxhost/openstackdb/nova/db/main"
	"testing"
	"time"
)

func TestInstanceFilters(t *testing.T) {
	conn := testutil.NewMySQLContainer(t, "nova-filters", "../sql/nova/schema.sql")
	testutil.SeedSQL(t, conn, `INSERT INTO instances (uuid, project_id, host, vm_state, deleted, created_at, deleted_at) VALUES
 ('live', 'p1', 'h1', 'active', 0, '2026-01-01', NULL),
 ('gone', 'p1', 'h1', 'deleted', 2, '2026-01-02', '2026-01-10'),
 ('other', 'p2', 'h2', 'active', 0, '2026-01-01', NULL),
 ('boundary', 'p1', 'h1', 'deleted', 4, '2025-12-01', '2026-01-01')`)
	q := nova.New(conn)
	ctx := context.Background()
	rows, err := q.InstanceGetAllByFilters(ctx, nova.InstanceFilters{ProjectIDs: []string{"p1"}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "live", rows[0].Uuid)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rows, err = q.InstanceGetAllByFilters(ctx, nova.InstanceFilters{ProjectIDs: []string{"p1"}, Deleted: db.IncludeDeleted, DeletedAfter: start, CreatedBefore: start.AddDate(0, 1, 0)})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	rows, err = q.InstanceGetAllByFilters(ctx, nova.InstanceFilters{Deleted: db.OnlyDeleted})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	rows, err = q.InstanceGetAllByFilters(ctx, nova.InstanceFilters{ProjectIDs: []string{}})
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = q.InstanceGetAllByFilters(ctx, nova.InstanceFilters{ProjectIDs: []string{"p1' OR 1=1 --"}})
	require.NoError(t, err)
	require.Empty(t, rows)
	instance, err := q.InstanceGetByUUID(ctx, "live")
	require.NoError(t, err)
	require.Equal(t, "live", instance.Uuid)
	_, err = q.InstanceGetByUUID(ctx, "gone")
	require.ErrorIs(t, err, sql.ErrNoRows)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = q.InstanceGetAllByFilters(canceled, nova.InstanceFilters{})
	require.ErrorIs(t, err, context.Canceled)
	tx, err := conn.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = q.WithTx(tx).InstanceGetByUUID(ctx, "live")
	require.NoError(t, err)
}

func TestVolumeFilters(t *testing.T) {
	conn := testutil.NewMySQLContainer(t, "volume-filters", "../sql/cinder/schema.sql")
	testutil.SeedSQL(t, conn, `INSERT INTO volumes (id, project_id, size, deleted, volume_type_id) VALUES
 ('v1', 'p1', 10, 0, 'type1'), ('v2', 'p1', 20, 1, 'type1'), ('v3', 'p2', 30, 0, 'type1')`)
	q := cinder.New(conn)
	rows, err := q.VolumeGetAllByFilters(context.Background(), cinder.VolumeFilters{ProjectIDs: []string{"p1"}, Deleted: db.IncludeDeleted})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	volume, err := q.VolumeGet(context.Background(), "v1")
	require.NoError(t, err)
	require.EqualValues(t, 10, volume.Size.Int32)
	_, err = q.VolumeGet(context.Background(), "v2")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestImageFilters(t *testing.T) {
	conn := testutil.NewMySQLContainer(t, "image-filters", "../sql/glance/schema.sql")
	testutil.SeedSQL(t, conn, `INSERT INTO images (id, owner, status, created_at, deleted, protected, min_disk, min_ram, visibility) VALUES
 ('i1', 'p1', 'active', '2026-01-01', 0, 0, 0, 0, 'private'),
 ('i2', 'p2', 'active', '2026-01-01', 0, 0, 0, 0, 'public')`)
	rows, err := glance.New(conn).ImageGetAllByFilters(context.Background(), glance.ImageFilters{ProjectIDs: []string{"p1"}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "i1", rows[0].ID)
}
