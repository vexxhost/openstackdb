package nova

import (
	"context"
	"database/sql"
	"fmt"
	db "github.com/vexxhost/openstackdb"
	"github.com/vexxhost/openstackdb/internal/filter"
)

// InstanceTypeOptions controls legacy instance_types reads. Archived selects
// shadow_instance_types; this explicit archive option extends the old Nova API.
type InstanceTypeOptions struct {
	Inactive bool
	Archived bool
}

// InstanceTypeRecord is the identity projection of a legacy instance type.
type InstanceTypeRecord struct {
	ID   int32
	Name sql.NullString
}

// InstanceTypeGetAll follows legacy Nova instance_type_get_all naming.
// Inactive includes soft-deleted types. Modern flavors live in nova/db/api.
func (q *Queries) InstanceTypeGetAll(ctx context.Context, opts InstanceTypeOptions) ([]InstanceTypeRecord, error) {
	table := "instance_types"
	if opts.Archived {
		table = "shadow_instance_types"
	}
	var b filter.Builder
	policy := db.ExcludeDeleted
	if opts.Inactive {
		policy = db.IncludeDeleted
	}
	if err := b.Deleted("deleted", policy); err != nil {
		return nil, err
	}
	query, args := b.SQL("SELECT id, name FROM " + table)
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]InstanceTypeRecord, 0)
	for rows.Next() {
		var r InstanceTypeRecord
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// InstanceExtraOptions selects the optional archived instance-extra table.
type InstanceExtraOptions struct{ Archived bool }

// InstanceExtraRecord exposes raw flavor JSON without interpreting its version.
type InstanceExtraRecord struct {
	InstanceUUID string
	Flavor       sql.NullString
}

// InstanceExtraGetByInstanceUUID follows Nova instance_extra_get_by_instance_uuid.
// Only flavor metadata is projected. Absent rows return sql.ErrNoRows.
func (q *Queries) InstanceExtraGetByInstanceUUID(ctx context.Context, uuid string, options ...InstanceExtraOptions) (InstanceExtraRecord, error) {
	opts, err := extraOptions(options)
	if err != nil {
		return InstanceExtraRecord{}, err
	}
	table := "instance_extra"
	if opts.Archived {
		table = "shadow_instance_extra"
	}
	var r InstanceExtraRecord
	err = q.db.QueryRowContext(ctx, "SELECT instance_uuid, flavor FROM "+table+" WHERE instance_uuid = ?", uuid).Scan(&r.InstanceUUID, &r.Flavor)
	return r, err
}

func extraOptions(options []InstanceExtraOptions) (InstanceExtraOptions, error) {
	if len(options) > 1 {
		return InstanceExtraOptions{}, fmt.Errorf("at most one instance extra option is allowed")
	}
	if len(options) == 1 {
		return options[0], nil
	}
	return InstanceExtraOptions{}, nil
}
