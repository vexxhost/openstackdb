package cinder

import (
	"context"
	"fmt"
)

// VolumeTypeOptions mirrors Cinder volume_type_get_all's inactive option.
type VolumeTypeOptions struct{ Inactive bool }

// VolumeTypeGetAllRow is the volume type identity projection.
type VolumeTypeGetAllRow = VolumeTypeGetAllCurrentRow

// VolumeTypeGetAll is retained as the default query text for query consumers.
const VolumeTypeGetAll = VolumeTypeGetAllCurrent

// VolumeTypeGetAll returns type identities, including deleted types when Inactive
// is true. Omitting options preserves the original current-type behavior.
func (q *Queries) VolumeTypeGetAll(ctx context.Context, options ...VolumeTypeOptions) ([]VolumeTypeGetAllRow, error) {
	if len(options) > 1 {
		return nil, fmt.Errorf("at most one volume type option is allowed")
	}
	if len(options) == 0 || !options[0].Inactive {
		return q.VolumeTypeGetAllCurrent(ctx)
	}
	rows, err := q.db.QueryContext(ctx, "SELECT id, name FROM volume_types")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]VolumeTypeGetAllRow, 0)
	for rows.Next() {
		var r VolumeTypeGetAllRow
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
