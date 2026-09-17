package openstackdb

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
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
