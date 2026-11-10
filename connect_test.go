package openstackdb

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConnectInvalidURLs(t *testing.T) {
	for _, u := range []string{"", "postgresql://user:pass@localhost/db"} {
		_, err := Connect(u)
		require.Error(t, err)
	}
}
func TestConnectCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ConnectContext(ctx, "mysql://user:pass@127.0.0.1:3306/db", ConnectionOptions{})
	require.ErrorIs(t, err, context.Canceled)
}
func TestConnectInvalidOptions(t *testing.T) {
	_, err := ConnectContext(context.Background(), "mysql://user:pass@localhost/db", ConnectionOptions{MaxOpenConns: -1})
	require.Error(t, err)
}
func TestOpenInvalidURLs(t *testing.T) {
	for _, u := range []string{"", "postgresql://user:pass@localhost/db"} {
		_, err := Open(u, ConnectionOptions{})
		require.Error(t, err)
	}
}
func TestOpenInvalidOptions(t *testing.T) {
	_, err := Open("mysql://user:pass@localhost/db", ConnectionOptions{MaxOpenConns: -1})
	require.Error(t, err)
}
func TestOpenUnreachableServer(t *testing.T) {
	db, err := Open("mysql://user:pass@127.0.0.1:1/db", ConnectionOptions{})
	require.NoError(t, err)
	defer db.Close()
	require.Equal(t, 5, db.Stats().MaxOpenConnections)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.Error(t, db.PingContext(ctx))
}
func TestOpenOptions(t *testing.T) {
	db, err := Open("mysql://user:pass@localhost/db", ConnectionOptions{MaxOpenConns: 7})
	require.NoError(t, err)
	defer db.Close()
	require.Equal(t, 7, db.Stats().MaxOpenConnections)
}
