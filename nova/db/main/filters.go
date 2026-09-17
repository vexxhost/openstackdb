package nova

import (
	"context"
	"database/sql"
	"fmt"
	db "github.com/vexxhost/openstackdb"
	"github.com/vexxhost/openstackdb/internal/filter"
	"time"
)

// InstanceRecord extends the current-resource projection with lifecycle metadata.
type InstanceRecord struct {
	InstanceGetAllRow
	CreatedAt sql.NullTime
	DeletedAt sql.NullTime
	Hostname  sql.NullString
	Flavor    sql.NullString
	Archived  bool
}

// InstanceFilters combines exact-match filters with AND. Values within a slice use OR.
// Nil slices are unrestricted; non-nil empty slices match no records.
// CreatedBefore and DeletedAfter form an optional half-open lifetime window.
// The zero deletion policy excludes soft-deleted records.
// ArchiveMode selects primary and/or shadow tables. This is a library extension.
type ArchiveMode uint8

const (
	ExcludeArchived ArchiveMode = iota
	IncludeArchived
	OnlyArchived
)

type InstanceFilters struct {
	Archive ArchiveMode
	// WithExtra joins instance_extra (or shadow_instance_extra) for raw flavor JSON.
	WithExtra     bool
	UUIDs         []string
	ProjectIDs    []string
	Hosts         []string
	VMStates      []string
	Deleted       db.Deleted
	CreatedBefore time.Time
	DeletedAfter  time.Time
	// IncludeStartBoundary includes records deleted exactly at DeletedAfter.
	IncludeStartBoundary bool
	// DeletedAtIsNull adds a deleted_at IS NULL predicate independently of Deleted.
	DeletedAtIsNull bool
}

// InstanceGetAllByFilters returns the resource projection for matching database records.
// Archive access and extra metadata are explicit options. It does not authorize
// requests or discover cells. Use WithTx for a consistent primary/shadow snapshot.
func (q *Queries) InstanceGetAllByFilters(ctx context.Context, f InstanceFilters) ([]InstanceRecord, error) {
	switch f.Archive {
	case ExcludeArchived:
		return q.instanceGetAllByFilters(ctx, f, false)
	case OnlyArchived:
		return q.instanceGetAllByFilters(ctx, f, true)
	case IncludeArchived:
		current, err := q.instanceGetAllByFilters(ctx, f, false)
		if err != nil {
			return nil, err
		}
		archived, err := q.instanceGetAllByFilters(ctx, f, true)
		if err != nil {
			return nil, err
		}
		return append(current, archived...), nil
	default:
		return nil, fmt.Errorf("invalid archive mode: %d", f.Archive)
	}
}

func (q *Queries) instanceGetAllByFilters(ctx context.Context, f InstanceFilters, archived bool) ([]InstanceRecord, error) {
	var b filter.Builder
	if err := b.Deleted("i.deleted", f.Deleted); err != nil {
		return nil, err
	}
	b.Strings("i.uuid", f.UUIDs)
	b.Strings("i.project_id", f.ProjectIDs)
	b.Strings("i.host", f.Hosts)
	b.Strings("i.vm_state", f.VMStates)
	lifetime := b.Lifetime
	if f.IncludeStartBoundary {
		lifetime = b.LifetimeIncludingStart
	}
	if err := lifetime("i.created_at", "i.deleted_at", f.DeletedAfter, f.CreatedBefore); err != nil {
		return nil, err
	}
	if f.DeletedAtIsNull {
		b.Clauses = append(b.Clauses, "i.deleted_at IS NULL")
	}
	table, extraTable := "instances", "instance_extra"
	if archived {
		table, extraTable = "shadow_instances", "shadow_instance_extra"
	}
	projection := `i.id, i.uuid, i.display_name, i.user_id, i.project_id, i.host,
 i.availability_zone, i.vm_state, i.power_state, i.task_state, i.memory_mb,
 i.vcpus, i.root_gb, i.ephemeral_gb, i.launched_at, i.terminated_at,
 i.instance_type_id, i.deleted, i.created_at, i.deleted_at, i.hostname`
	flavor := "NULL"
	join := ""
	if f.WithExtra {
		flavor = "e.flavor"
		join = " LEFT JOIN " + extraTable + " e ON e.instance_uuid = i.uuid"
	}
	query, args := b.SQL("SELECT " + projection + ", " + flavor + " FROM " + table + " i" + join)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]InstanceRecord, 0)
	for rows.Next() {
		var i InstanceRecord
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
			&i.CreatedAt,
			&i.DeletedAt,
			&i.Hostname,
			&i.Flavor,
		); err != nil {
			return nil, err
		}
		i.Archived = archived
		result = append(result, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// InstanceGetByUUID returns one undeleted resource, or sql.ErrNoRows when absent.
func (q *Queries) InstanceGetByUUID(ctx context.Context, id string) (InstanceRecord, error) {
	rows, err := q.InstanceGetAllByFilters(ctx, InstanceFilters{UUIDs: []string{id}})
	if err != nil {
		return InstanceRecord{}, err
	}
	if len(rows) == 0 {
		return InstanceRecord{}, sql.ErrNoRows
	}
	return rows[0], nil
}
