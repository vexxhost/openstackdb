package nova

import (
	"context"
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	db "github.com/vexxhost/openstackdb"
	"testing"
)

func TestInstanceGetAllByFilters(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	mock.ExpectQuery(`WHERE deleted = 0 AND .* IN \(\?,\?\) AND project_id IN \(\?\)`).WithArgs("one", "two", "project").WillReturnError(sql.ErrConnDone)
	_, err = New(conn).InstanceGetAllByFilters(context.Background(), InstanceFilters{UUIDs: []string{"one", "two"}, ProjectIDs: []string{"project"}})
	require.ErrorIs(t, err, sql.ErrConnDone)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestInvalidDeletionPolicy(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	_, err = New(conn).InstanceGetAllByFilters(context.Background(), InstanceFilters{Deleted: db.Deleted(99)})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
