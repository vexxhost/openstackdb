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
	keystone "github.com/vexxhost/openstackdb/keystone/db"
	novaapi "github.com/vexxhost/openstackdb/nova/db/api"
	nova "github.com/vexxhost/openstackdb/nova/db/main"
	"testing"
	"time"
)

func TestInstanceHistoryAndMetadata(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "instance-history", "../sql/nova/schema.sql")
	testutil.SeedSQL(t, conn,
		`INSERT INTO instances (uuid, project_id, created_at, deleted_at, deleted, hostname, instance_type_id) VALUES
   ('current', 'p1', '2026-01-02', NULL, 0, 'current-host', 7),
   ('deleted', 'p1', '2026-01-03', '2026-01-05', 2, 'deleted-host', 8),
   ('boundary', 'p1', '2025-12-01', '2026-01-01', 3, 'boundary-host', 7),
   ('future', 'p1', '2026-02-01', NULL, 0, 'future-host', 7),
   ('other', 'p2', '2026-01-01', NULL, 0, 'other-host', 7)`,
		`INSERT INTO shadow_instances (uuid, project_id, created_at, deleted_at, deleted, hostname, instance_type_id) VALUES
   ('archived', 'p1', '2026-01-04', '2026-01-06', 9, 'archived-host', 53),
   ('missing-extra', 'p1', '2026-01-05', '2026-01-07', 10, 'missing-host', 59),
   ('old', 'p1', '2025-12-01', '2025-12-31', 11, 'old-host', 59)`,
		`INSERT INTO instance_extra (instance_uuid, flavor) VALUES ('current', '{"cur":{"nova_object.data":{"name":"modern"}}}')`,
		`INSERT INTO shadow_instance_extra (instance_uuid, flavor) VALUES ('archived', '{"cur":{"nova_object.data":{"name":"legacy"}}}')`,
		`INSERT INTO instance_types (id,name,deleted) VALUES (7,'current-type',0), (8,'removed-type',8)`,
		`INSERT INTO shadow_instance_types (id,name,deleted) VALUES (53,'archived-type',53), (59,'fallback-type',59)`)
	q := nova.New(conn)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := nova.InstanceFilters{ProjectIDs: []string{"p1"}, Deleted: db.IncludeDeleted, Archive: nova.IncludeArchived, WithExtra: true, DeletedAfter: start, CreatedBefore: start.AddDate(0, 1, 0), IncludeStartBoundary: true}
	rows, err := q.InstanceGetAllByFilters(ctx, f)
	require.NoError(t, err)
	require.Len(t, rows, 5)
	byID := map[string]nova.InstanceRecord{}
	for _, row := range rows {
		byID[row.Uuid] = row
	}
	require.Equal(t, "current-host", byID["current"].Hostname.String)
	require.Contains(t, byID["current"].Flavor.String, "modern")
	require.False(t, byID["current"].Archived)
	require.True(t, byID["archived"].Archived)
	require.Contains(t, byID["archived"].Flavor.String, "legacy")
	require.False(t, byID["missing-extra"].Flavor.Valid)
	require.Equal(t, start.AddDate(0, 0, 3), byID["archived"].CreatedAt.Time)
	require.Equal(t, start.AddDate(0, 0, 5), byID["archived"].DeletedAt.Time)
	require.Equal(t, start, byID["boundary"].DeletedAt.Time)
	require.False(t, byID["current"].DeletedAt.Valid)
	f.IncludeStartBoundary = false
	rows, err = q.InstanceGetAllByFilters(ctx, f)
	require.NoError(t, err)
	require.Len(t, rows, 4)
	f.Archive = nova.OnlyArchived
	rows, err = q.InstanceGetAllByFilters(ctx, f)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	f.ProjectIDs = []string{}
	rows, err = q.InstanceGetAllByFilters(ctx, f)
	require.NoError(t, err)
	require.Empty(t, rows)
	// Timestamp-only active selection is independent of the soft-delete flag.
	testutil.SeedSQL(t, conn, `INSERT INTO instances(uuid,project_id,deleted,deleted_at) VALUES ('flagged','p1',99,NULL)`)
	rows, err = q.InstanceGetAllByFilters(ctx, nova.InstanceFilters{ProjectIDs: []string{"p1"}, Deleted: db.IncludeDeleted, DeletedAtIsNull: true})
	require.NoError(t, err)
	require.Len(t, rows, 3)
	extra, err := q.InstanceExtraGetByInstanceUUID(ctx, "archived", nova.InstanceExtraOptions{Archived: true})
	require.NoError(t, err)
	require.Contains(t, extra.Flavor.String, "legacy")
	_, err = q.InstanceExtraGetByInstanceUUID(ctx, "missing-extra", nova.InstanceExtraOptions{Archived: true})
	require.ErrorIs(t, err, sql.ErrNoRows)
	types, err := q.InstanceTypeGetAll(ctx, nova.InstanceTypeOptions{})
	require.NoError(t, err)
	require.Len(t, types, 1)
	types, err = q.InstanceTypeGetAll(ctx, nova.InstanceTypeOptions{Inactive: true})
	require.NoError(t, err)
	require.Len(t, types, 2)
	types, err = q.InstanceTypeGetAll(ctx, nova.InstanceTypeOptions{Inactive: true, Archived: true})
	require.NoError(t, err)
	require.Len(t, types, 2)
	// Legacy table absence must not affect ordinary primary-table reads.
	testutil.SeedSQL(t, conn, "DROP TABLE shadow_instances", "DROP TABLE shadow_instance_extra", "DROP TABLE instance_extra")
	_, err = q.InstanceGetAllByFilters(ctx, nova.InstanceFilters{})
	require.NoError(t, err)
	_, err = q.InstanceGetAllByFilters(ctx, nova.InstanceFilters{Archive: nova.IncludeArchived})
	require.Error(t, err) // Never silently return an incomplete historical result.
}

func TestVolumeHistoryAndInactiveTypes(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "volume-history", "../sql/cinder/schema.sql")
	testutil.SeedSQL(t, conn,
		`INSERT INTO volume_types(id,name,deleted) VALUES ('active','fast',0), ('gone','legacy',1)`,
		`INSERT INTO volumes(id,project_id,size,volume_type_id,created_at,deleted_at,deleted) VALUES
   ('current','p1',10,'active','2026-01-01',NULL,0),
   ('deleted','p1',20,'gone','2026-01-02','2026-01-04',1),
   ('default','p1',30,NULL,'2026-01-03',NULL,0),
   ('boundary','p1',40,'gone','2025-12-01','2026-01-01',1),
   ('other','p2',50,'active','2026-01-01',NULL,0)`)
	q := cinder.New(conn)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rows, err := q.VolumeGetAllByFilters(ctx, cinder.VolumeFilters{ProjectIDs: []string{"p1"}, Deleted: db.IncludeDeleted, DeletedAfter: start, CreatedBefore: start.AddDate(0, 1, 0), IncludeStartBoundary: true})
	require.NoError(t, err)
	require.Len(t, rows, 4)
	for _, row := range rows {
		if row.ID == "default" {
			require.False(t, row.VolumeTypeID.Valid)
		}
		if row.ID == "deleted" {
			require.EqualValues(t, 20, row.Size.Int32)
			require.Equal(t, start.AddDate(0, 0, 3), row.DeletedAt.Time)
		}
	}
	types, err := q.VolumeTypeGetAll(ctx)
	require.NoError(t, err)
	require.Len(t, types, 1)
	types, err = q.VolumeTypeGetAll(ctx, cinder.VolumeTypeOptions{Inactive: true})
	require.NoError(t, err)
	require.Len(t, types, 2)
	rows, err = q.VolumeGetAllByFilters(ctx, cinder.VolumeFilters{ProjectIDs: []string{"p1"}, Deleted: db.IncludeDeleted, DeletedAtIsNull: true})
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestImageHistoryTimestamps(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "image-history", "../sql/glance/schema.sql")
	testutil.SeedSQL(t, conn, `INSERT INTO images(id,owner,size,status,created_at,deleted_at,deleted,protected,min_disk,min_ram,visibility) VALUES
 ('current','p1',NULL,'active','2026-01-01',NULL,0,0,0,0,'private'),
 ('deleted','p1',5368709120,'deleted','2026-01-02','2026-01-03',1,0,0,0,'private'),
 ('boundary','p1',1073741824,'deleted','2025-12-01','2026-01-01',1,0,0,0,'private'),
 ('other','p2',1,'active','2026-01-01',NULL,0,0,0,0,'private')`)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := glance.New(conn)
	rows, err := q.ImageGetAllByFilters(ctx, glance.ImageFilters{ProjectIDs: []string{"p1"}, Deleted: db.IncludeDeleted, DeletedAfter: start, CreatedBefore: start.AddDate(0, 1, 0), IncludeStartBoundary: true})
	require.NoError(t, err)
	require.Len(t, rows, 3)
	for _, row := range rows {
		if row.ID == "deleted" {
			require.EqualValues(t, 5368709120, row.Size.Int64)
			require.Equal(t, start.AddDate(0, 0, 2), row.DeletedAt.Time)
		}
		if row.ID == "current" {
			require.False(t, row.Size.Valid)
			require.False(t, row.DeletedAt.Valid)
		}
	}
	rows, err = q.ImageGetAllByFilters(ctx, glance.ImageFilters{ProjectIDs: []string{"p1"}, Deleted: db.IncludeDeleted, DeletedAtIsNull: true})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestProjectLookupAndAPIFlavors(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "project-lookup", "../sql/keystone/schema.sql")
	testutil.SeedSQL(t, conn, `INSERT INTO project(id,name,enabled,domain_id,is_domain) VALUES ('p1','disabled',0,'d1',0)`)
	project, err := keystone.New(conn).GetProject(ctx, "p1")
	require.NoError(t, err)
	require.Equal(t, "disabled", project.Name)
	_, err = keystone.New(conn).GetProject(ctx, "absent")
	require.ErrorIs(t, err, sql.ErrNoRows)
	apiConn := testutil.NewMySQLContainer(t, "api-flavors", "../sql/nova_api/schema.sql")
	testutil.SeedSQL(t, apiConn, `INSERT INTO flavors(id,flavorid,name,memory_mb,vcpus,root_gb,ephemeral_gb,swap,rxtx_factor,disabled,is_public) VALUES (7,'f1','modern',1024,1,10,0,0,1,0,1)`)
	flavors, err := novaapi.New(apiConn).FlavorGetAll(ctx)
	require.NoError(t, err)
	require.Len(t, flavors, 1)
	require.EqualValues(t, 7, flavors[0].ID)
	require.Equal(t, "modern", flavors[0].Name)
}
