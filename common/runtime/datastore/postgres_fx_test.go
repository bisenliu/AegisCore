package datastore

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/resources"
)

func TestNewPostgresClosesPoolWhenStartPingFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().(*net.TCPAddr)
	require.NoError(t, listener.Close())
	lc := fxtest.NewLifecycle(t)
	db, err := NewPostgres(lc, zap.NewNop(), testPrimaryDB, resources.PostgresConfig{
		Host: "127.0.0.1", Port: addr.Port, Username: "aegiscore", DBName: "aegiscore_user", SSLMode: "disable",
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = lc.Start(ctx)
	require.ErrorContains(t, err, "ping postgres primary_db")
	require.ErrorContains(t, db.PingContext(ctx), "database is closed")
}
