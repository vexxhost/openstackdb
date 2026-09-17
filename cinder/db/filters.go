package cinder

import (
	"context"
	"database/sql"
	db "github.com/vexxhost/openstackdb"
	"github.com/vexxhost/openstackdb/internal/filter"
	"time"
)

// VolumeFilters combines exact-match filters with AND. Values within a slice use OR.
// Nil slices are unrestricted; non-nil empty slices match no records.
// CreatedBefore and DeletedAfter form an optional half-open lifetime window.
// The zero deletion policy excludes soft-deleted records.
type VolumeFilters struct {
	IDs           []string
	ProjectIDs    []string
	Statuses      []string
	VolumeTypeIDs []string
	Deleted       db.Deleted
	CreatedBefore time.Time
	DeletedAfter  time.Time
}

// VolumeGetAllByFilters returns the resource projection for matching database records.
// It does not perform OpenStack authorization, cell discovery, or archive searches.
func (q *Queries) VolumeGetAllByFilters(ctx context.Context, f VolumeFilters) ([]VolumeGetAllRow, error) {
	var b filter.Builder
	if err := b.Deleted("deleted", f.Deleted); err != nil {
		return nil, err
	}
	b.Strings("id", f.IDs)
	b.Strings("project_id", f.ProjectIDs)
	b.Strings("status", f.Statuses)
	b.Strings("volume_type_id", f.VolumeTypeIDs)
	if err := b.Lifetime("created_at", "deleted_at", f.DeletedAfter, f.CreatedBefore); err != nil {
		return nil, err
	}
	query, args := b.SQL(`SELECT id, project_id, size, volume_type_id, status, created_at, deleted_at
FROM volumes`)
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]VolumeGetAllRow, 0)
	for rows.Next() {
		var i VolumeGetAllRow
		if err := rows.Scan(
			&i.ID,
			&i.ProjectID,
			&i.Size,
			&i.VolumeTypeID,
			&i.Status,
			&i.CreatedAt,
			&i.DeletedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// VolumeGet returns one undeleted resource, or sql.ErrNoRows when absent.
func (q *Queries) VolumeGet(ctx context.Context, id string) (VolumeGetAllRow, error) {
	rows, err := q.VolumeGetAllByFilters(ctx, VolumeFilters{IDs: []string{id}})
	if err != nil {
		return VolumeGetAllRow{}, err
	}
	if len(rows) == 0 {
		return VolumeGetAllRow{}, sql.ErrNoRows
	}
	return rows[0], nil
}
