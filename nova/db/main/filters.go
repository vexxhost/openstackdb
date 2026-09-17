package nova

import (
	"context"
	"database/sql"
	db "github.com/vexxhost/openstackdb"
	"github.com/vexxhost/openstackdb/internal/filter"
	"time"
)

// InstanceFilters combines exact-match filters with AND. Values within a slice use OR.
// Nil slices are unrestricted; non-nil empty slices match no records.
// CreatedBefore and DeletedAfter form an optional half-open lifetime window.
// The zero deletion policy excludes soft-deleted records.
type InstanceFilters struct {
	UUIDs         []string
	ProjectIDs    []string
	Hosts         []string
	VMStates      []string
	Deleted       db.Deleted
	CreatedBefore time.Time
	DeletedAfter  time.Time
}

// InstanceGetAllByFilters returns the resource projection for matching database records.
// It does not perform OpenStack authorization, cell discovery, or archive searches.
func (q *Queries) InstanceGetAllByFilters(ctx context.Context, f InstanceFilters) ([]InstanceGetAllRow, error) {
	var b filter.Builder
	if err := b.Deleted("deleted", f.Deleted); err != nil {
		return nil, err
	}
	b.Strings("uuid", f.UUIDs)
	b.Strings("project_id", f.ProjectIDs)
	b.Strings("host", f.Hosts)
	b.Strings("vm_state", f.VMStates)
	if err := b.Lifetime("created_at", "deleted_at", f.DeletedAfter, f.CreatedBefore); err != nil {
		return nil, err
	}
	query, args := b.SQL(`SELECT 
    id,
    uuid,
    display_name,
    user_id,
    project_id,
    host,
    availability_zone,
    vm_state,
    power_state,
    task_state,
    memory_mb,
    vcpus,
    root_gb,
    ephemeral_gb,
    launched_at,
    terminated_at,
    instance_type_id,
    deleted
FROM instances`)
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]InstanceGetAllRow, 0)
	for rows.Next() {
		var i InstanceGetAllRow
		if err := rows.Scan(
			&i.ID,
			&i.Uuid,
			&i.DisplayName,
			&i.UserID,
			&i.ProjectID,
			&i.Host,
			&i.AvailabilityZone,
			&i.VmState,
			&i.PowerState,
			&i.TaskState,
			&i.MemoryMb,
			&i.Vcpus,
			&i.RootGb,
			&i.EphemeralGb,
			&i.LaunchedAt,
			&i.TerminatedAt,
			&i.InstanceTypeID,
			&i.Deleted,
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

// InstanceGetByUUID returns one undeleted resource, or sql.ErrNoRows when absent.
func (q *Queries) InstanceGetByUUID(ctx context.Context, id string) (InstanceGetAllRow, error) {
	rows, err := q.InstanceGetAllByFilters(ctx, InstanceFilters{UUIDs: []string{id}})
	if err != nil {
		return InstanceGetAllRow{}, err
	}
	if len(rows) == 0 {
		return InstanceGetAllRow{}, sql.ErrNoRows
	}
	return rows[0], nil
}
