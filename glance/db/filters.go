package glance

import (
	"context"
	"database/sql"
	db "github.com/vexxhost/openstackdb"
	"github.com/vexxhost/openstackdb/internal/filter"
	"time"
)

// ImageRecord extends the current-resource projection with lifecycle metadata.
type ImageRecord struct {
	ImageGetAllRow
	DeletedAt sql.NullTime
}

// ImageFilters combines exact-match filters with AND. Values within a slice use OR.
// Nil slices are unrestricted; non-nil empty slices match no records.
// CreatedBefore and DeletedAfter form an optional half-open lifetime window.
// The zero deletion policy excludes soft-deleted records.
type ImageFilters struct {
	IDs           []string
	ProjectIDs    []string
	Statuses      []string
	Visibilities  []string
	Deleted       db.Deleted
	CreatedBefore time.Time
	DeletedAfter  time.Time
	// IncludeStartBoundary includes records deleted exactly at DeletedAfter.
	IncludeStartBoundary bool
	// DeletedAtIsNull adds a deleted_at IS NULL predicate independently of Deleted.
	DeletedAtIsNull bool
}

// ImageGetAllByFilters returns the resource projection for matching database records.
// It does not perform OpenStack authorization, cell discovery, or archive searches.
func (q *Queries) ImageGetAllByFilters(ctx context.Context, f ImageFilters) ([]ImageRecord, error) {
	var b filter.Builder
	if err := b.Deleted("deleted", f.Deleted); err != nil {
		return nil, err
	}
	b.Strings("id", f.IDs)
	b.Strings("owner", f.ProjectIDs)
	b.Strings("status", f.Statuses)
	b.Strings("visibility", f.Visibilities)
	lifetime := b.Lifetime
	if f.IncludeStartBoundary {
		lifetime = b.LifetimeIncludingStart
	}
	if err := lifetime("created_at", "deleted_at", f.DeletedAfter, f.CreatedBefore); err != nil {
		return nil, err
	}
	if f.DeletedAtIsNull {
		b.Clauses = append(b.Clauses, "deleted_at IS NULL")
	}
	query, args := b.SQL(`SELECT
    id,
    name,
    size,
    status,
    owner,
    visibility,
    disk_format,
    container_format,
    checksum,
    created_at,
    updated_at,
    min_disk,
    min_ram,
    protected,
    virtual_size,
    os_hidden,
    os_hash_algo,
    os_hash_value,
    deleted_at
FROM
    images`)
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ImageRecord, 0)
	for rows.Next() {
		var i ImageRecord
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Size,
			&i.Status,
			&i.Owner,
			&i.Visibility,
			&i.DiskFormat,
			&i.ContainerFormat,
			&i.Checksum,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.MinDisk,
			&i.MinRam,
			&i.Protected,
			&i.VirtualSize,
			&i.OsHidden,
			&i.OsHashAlgo,
			&i.OsHashValue,
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

// ImageGet returns one undeleted resource, or sql.ErrNoRows when absent.
func (q *Queries) ImageGet(ctx context.Context, id string) (ImageRecord, error) {
	rows, err := q.ImageGetAllByFilters(ctx, ImageFilters{IDs: []string{id}})
	if err != nil {
		return ImageRecord{}, err
	}
	if len(rows) == 0 {
		return ImageRecord{}, sql.ErrNoRows
	}
	return rows[0], nil
}
