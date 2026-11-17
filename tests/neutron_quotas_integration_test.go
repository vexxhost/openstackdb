//go:build integration

package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vexxhost/openstackdb/internal/testutil"
	neutron "github.com/vexxhost/openstackdb/neutron/db"
)

func TestNeutronActiveReservations(t *testing.T) {
	if _, err := os.Stat("/var/run/docker.sock"); err != nil && os.Getenv("DOCKER_HOST") == "" {
		t.Skip("Docker unavailable; MariaDB integration requires a Docker host")
	}
	testutil.SkipIfNoDocker(t)
	db := testutil.NewMySQLContainer(t, "neutron-quotas", "../schema/neutron/schema.sql")
	cutoff := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resources := []string{"floatingip", "router", "network", "port", "security_group", "security_group_rule", "subnet", "subnetpool", "rbac_policy"}
	for i, r := range resources {
		for j, entry := range []struct {
			expiration any
			amount     any
		}{{cutoff, 2}, {cutoff.Add(time.Minute), 3}, {cutoff.Add(-time.Second), 100}, {nil, 100}} {
			id := fmt.Sprintf("r-%d-%d", i, j)
			_, err := db.ExecContext(ctx, "INSERT INTO reservations (id, project_id, expiration) VALUES (?, ?, ?)", id, "reservation-only", entry.expiration)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, "INSERT INTO resourcedeltas (resource, reservation_id, amount) VALUES (?, ?, ?)", r, id, entry.amount)
			require.NoError(t, err)
		}
	}
	for i, r := range resources {
		for j, owner := range []any{"", nil} {
			id := fmt.Sprintf("system-%d-%d", i, j)
			_, err := db.ExecContext(ctx, "INSERT INTO reservations (id, project_id, expiration) VALUES (?, ?, ?)", id, owner, cutoff)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, "INSERT INTO resourcedeltas (resource, reservation_id, amount) VALUES (?, ?, 40)", r, id)
			require.NoError(t, err)
		}
	}
	q := neutron.New(db)
	rows, err := q.GetActiveReservationsByProject(ctx, sql.NullTime{Time: cutoff, Valid: true})
	require.NoError(t, err)
	require.Len(t, rows, 9)
	var actualResources []string
	for _, row := range rows {
		require.True(t, row.ProjectID.Valid)
		actualResources = append(actualResources, row.Resource)
		require.Equal(t, "reservation-only", row.ProjectID.String)
		require.Equal(t, int64(5), row.Reserved)
		require.Zero(t, row.InvalidAmounts)
	}
	require.ElementsMatch(t, resources, actualResources)
	// Cover malformed amounts and signed aggregation for every resource on MariaDB.
	expectedReservations := append([]neutron.GetActiveReservationsByProjectRow{}, rows...)
	for i, r := range resources {
		for j, entry := range []struct {
			owner    string
			amounts  []any
			reserved int64
			invalid  int64
		}{
			{"mixed", []any{7, -2, nil}, 5, 2},
			{"negative", []any{-5, 2}, -3, 1},
			{"null", []any{nil, nil}, 0, 2},
			{"zero", []any{0}, 0, 0},
			{"large", []any{2147483647, 1}, 2147483648, 0},
		} {
			for k, amount := range entry.amounts {
				id := fmt.Sprintf("amount-%d-%d-%d", i, j, k)
				_, err := db.ExecContext(ctx, "INSERT INTO reservations VALUES (?, ?, ?)", id, entry.owner, cutoff)
				require.NoError(t, err)
				_, err = db.ExecContext(ctx, "INSERT INTO resourcedeltas VALUES (?, ?, ?)", r, id, amount)
				require.NoError(t, err)
			}
			expectedReservations = append(expectedReservations, neutron.GetActiveReservationsByProjectRow{
				ProjectID: sql.NullString{String: entry.owner, Valid: true}, Resource: r,
				Reserved: entry.reserved, InvalidAmounts: entry.invalid,
			})
		}
		for j, expiration := range []any{cutoff.Add(-time.Second), nil} {
			id := fmt.Sprintf("inactive-invalid-%d-%d", i, j)
			_, err := db.ExecContext(ctx, "INSERT INTO reservations VALUES (?, 'mixed', ?)", id, expiration)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, "INSERT INTO resourcedeltas VALUES (?, ?, -100)", r, id)
			require.NoError(t, err)
		}
	}
	_, err = db.ExecContext(ctx, "INSERT INTO reservations VALUES ('unsupported', 'extension-owner', ?)", cutoff)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO resourcedeltas VALUES ('extension', 'unsupported', 10)")
	require.NoError(t, err)
	rows, err = q.GetActiveReservationsByProject(ctx, sql.NullTime{Time: cutoff, Valid: true})
	require.NoError(t, err)
	require.ElementsMatch(t, expectedReservations, rows)
	rows, err = q.GetActiveReservationsByProject(ctx, sql.NullTime{})
	require.NoError(t, err)
	require.Empty(t, rows)

	// Each table owns its counts without requiring parent rows or quota overrides.
	// Non-ACTIVE objects are still counted; no status or quota join is added.
	inserts := []string{
		"INSERT INTO floatingips (id, project_id, floating_ip_address, floating_network_id, floating_port_id, status, standard_attr_id) VALUES (?, ?, '192.0.2.1', 'parent-network', 'parent-port', 'DOWN', 1)",
		"INSERT INTO networks (id, project_id, status, standard_attr_id) VALUES (?, ?, 'DOWN', 1)",
		"INSERT INTO ports (id, project_id, network_id, mac_address, admin_state_up, status, device_id, device_owner, standard_attr_id) VALUES (?, ?, 'parent-network', '00:00:00:00:00:01', 0, 'DOWN', '', '', 1)",
		"INSERT INTO routers (id, project_id, status, standard_attr_id) VALUES (?, ?, 'DOWN', 1)",
		"INSERT INTO securitygroups (id, project_id, standard_attr_id) VALUES (?, ?, 1)",
		"INSERT INTO securitygrouprules (id, project_id, security_group_id, standard_attr_id) VALUES (?, ?, 'parent-group', 1)",
		"INSERT INTO subnets (id, project_id, network_id, ip_version, cidr, standard_attr_id) VALUES (?, ?, 'parent-network', 4, '192.0.2.0/24', 1)",
		"INSERT INTO networkrbacs (id, project_id, object_id, target_project, action) VALUES (?, ?, 'parent-network', 'target-owner', 'access_as_shared')",
		"INSERT INTO subnetpools (id, project_id, ip_version, default_prefixlen, min_prefixlen, max_prefixlen, standard_attr_id) VALUES (?, ?, 4, 24, 8, 32, 1)",
	}
	for i, insert := range inserts {
		for j, owner := range []any{"a", "a", "b", "", nil} {
			_, err := db.ExecContext(ctx, insert, fmt.Sprintf("object-%d-%d", i, j), owner)
			require.NoError(t, err)
		}
	}
	usage, err := q.GetResourceCountsByProject(ctx)
	require.NoError(t, err)
	expected := []neutron.GetResourceCountsByProjectRow{}
	for _, r := range resources {
		expected = append(expected,
			neutron.GetResourceCountsByProjectRow{ProjectID: sql.NullString{String: "a", Valid: true}, Resource: r, Cnt: 2},
			neutron.GetResourceCountsByProjectRow{ProjectID: sql.NullString{String: "b", Valid: true}, Resource: r, Cnt: 1})
	}
	require.ElementsMatch(t, expected, usage)

	// GetQuotas keeps its older owner and raw-row contracts.
	expectedLimits := []neutron.GetQuotasRow{}
	for i, owner := range []any{"quota-owner", "", nil} {
		for j, limit := range []any{0, -1, nil, 42, 42} {
			_, err := db.ExecContext(ctx, "INSERT INTO quotas VALUES (?, ?, 'port', ?)", fmt.Sprintf("quota-%d-%d", i, j), owner, limit)
			require.NoError(t, err)
			if owner != nil {
				value := sql.NullInt32{}
				if limit != nil {
					value = sql.NullInt32{Int32: int32(limit.(int)), Valid: true}
				}
				expectedLimits = append(expectedLimits, neutron.GetQuotasRow{
					ProjectID: sql.NullString{String: owner.(string), Valid: true},
					Resource:  sql.NullString{String: "port", Valid: true}, Limit: value,
				})
			}
		}
	}
	limits, err := q.GetQuotas(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedLimits, limits)
}
