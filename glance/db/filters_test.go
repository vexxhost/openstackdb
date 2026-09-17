package glance

import (
	"context"
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	db "github.com/vexxhost/openstackdb"
	"testing"
)

func TestImageGetAllByFilters(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	query := New(conn)
	mock.ExpectQuery(`WHERE deleted = 0 AND .* IN \(\?,\?\) AND owner IN \(\?\)`).WithArgs("one", "two", "project").WillReturnError(sql.ErrConnDone)
	_, err = query.ImageGetAllByFilters(context.Background(), ImageFilters{IDs: []string{"one", "two"}, ProjectIDs: []string{"project"}})
	require.ErrorIs(t, err, sql.ErrConnDone)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestInvalidDeletionPolicy(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	_, err = New(conn).ImageGetAllByFilters(context.Background(), ImageFilters{Deleted: db.Deleted(99)})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
