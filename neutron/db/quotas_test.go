package neutron

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestActiveReservationQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { mock.ExpectClose(); require.NoError(t, db.Close()) })
	cutoff := sql.NullTime{Time: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC), Valid: true}
	result := sqlmock.NewRows([]string{"project_id", "resource", "reserved", "invalid_amounts"})
	resources := []string{"floatingip", "router", "network", "port", "security_group", "security_group_rule", "subnet", "subnetpool", "rbac_policy"}
	for _, r := range resources {
		result.AddRow("reservation-only", r, 5, 0)
	}
	result.AddRow(nil, "router", 0, 1)
	mock.ExpectQuery(regexp.QuoteMeta(GetActiveReservationsByProject)).WithArgs(cutoff).WillReturnRows(result).RowsWillBeClosed()
	rows, err := New(db).GetActiveReservationsByProject(context.Background(), cutoff)
	require.NoError(t, err)
	require.Len(t, rows, 10)
	for i, r := range resources {
		require.Equal(t, r, rows[i].Resource)
		require.Equal(t, int64(5), rows[i].Reserved)
		require.Zero(t, rows[i].InvalidAmounts)
	}
	require.False(t, rows[9].ProjectID.Valid)
	require.Equal(t, int64(1), rows[9].InvalidAmounts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReservationQueryErrors(t *testing.T) {
	for _, kind := range []string{"query", "scan", "iteration", "timeout"} {
		t.Run(kind, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { mock.ExpectClose(); require.NoError(t, db.Close()) })
			exp := mock.ExpectQuery(regexp.QuoteMeta(GetActiveReservationsByProject))
			rows := sqlmock.NewRows([]string{"project_id", "resource", "reserved", "invalid_amounts"})
			switch kind {
			case "query":
				exp.WillReturnError(errors.New("read failure"))
			case "scan":
				exp.WillReturnRows(rows.AddRow("p", "port", "malformed", 0)).RowsWillBeClosed()
			case "iteration":
				exp.WillReturnRows(rows.AddRow("p", "port", 1, 0).RowError(0, errors.New("interrupted"))).RowsWillBeClosed()
			case "timeout":
				exp.WillDelayFor(time.Second).WillReturnRows(rows)
			}
			timeout := time.Second
			if kind == "timeout" {
				timeout = 10 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			_, err = New(db).GetActiveReservationsByProject(ctx, sql.NullTime{Time: time.Now().UTC(), Valid: true})
			require.Error(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// sqlmock bypasses filtering to verify the generated API preserves ownership
// validity for defensive callers, including explicit empty strings versus NULL.
func TestQuotaAggregateOwnerValidity(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { mock.ExpectClose(); require.NoError(t, db.Close()) })
	mock.ExpectQuery(regexp.QuoteMeta(GetResourceCountsByProject)).WillReturnRows(
		sqlmock.NewRows([]string{"project_id", "resource", "cnt"}).
			AddRow("a", "port", 12).AddRow("", "port", 40).AddRow(nil, "port", 2))
	usage, err := New(db).GetResourceCountsByProject(context.Background())
	require.NoError(t, err)
	require.Len(t, usage, 3)
	require.Equal(t, sql.NullString{String: "a", Valid: true}, usage[0].ProjectID)
	require.Equal(t, sql.NullString{Valid: true}, usage[1].ProjectID)
	require.Equal(t, sql.NullString{}, usage[2].ProjectID)
	mock.ExpectQuery(regexp.QuoteMeta(GetActiveReservationsByProject)).WithArgs(sqlmock.AnyArg()).WillReturnRows(
		sqlmock.NewRows([]string{"project_id", "resource", "reserved", "invalid_amounts"}).
			AddRow("a", "port", 4, 0).AddRow("", "port", 40, 0).AddRow(nil, "port", 2, 0))
	reserved, err := New(db).GetActiveReservationsByProject(context.Background(), sql.NullTime{Time: time.Now().UTC(), Valid: true})
	require.NoError(t, err)
	require.Len(t, reserved, 3)
	for i := range usage {
		require.Equal(t, usage[i].ProjectID, reserved[i].ProjectID)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReservationNullCutoffAndSignedScan(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { mock.ExpectClose(); require.NoError(t, db.Close()) })
	columns := []string{"project_id", "resource", "reserved", "invalid_amounts"}
	mock.ExpectQuery(regexp.QuoteMeta(GetActiveReservationsByProject)).WithArgs(nil).
		WillReturnRows(sqlmock.NewRows(columns)).RowsWillBeClosed()
	rows, err := New(db).GetActiveReservationsByProject(context.Background(), sql.NullTime{})
	require.NoError(t, err)
	require.Empty(t, rows)
	cutoff := sql.NullTime{Time: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC), Valid: true}
	mock.ExpectQuery(regexp.QuoteMeta(GetActiveReservationsByProject)).WithArgs(cutoff).
		WillReturnRows(sqlmock.NewRows(columns).AddRow("owner", "port", -3, 2).
			AddRow("owner", "network", int64(2147483648), 0)).RowsWillBeClosed()
	rows, err = New(db).GetActiveReservationsByProject(context.Background(), cutoff)
	require.NoError(t, err)
	require.Equal(t, []GetActiveReservationsByProjectRow{
		{ProjectID: sql.NullString{String: "owner", Valid: true}, Resource: "port", Reserved: -3, InvalidAmounts: 2},
		{ProjectID: sql.NullString{String: "owner", Valid: true}, Resource: "network", Reserved: 2147483648},
	}, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}
