package logger

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
)

func TestNewLoggerDoesNotReplaceDefaultLogger(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	setDefaultLoggerForTest(t, zap.New(core))
	lc := fxtest.NewLifecycle(t)

	created, err := NewLogger(lc, &config.Config{App: config.AppConfig{Name: "user-service", Environment: "test"}})
	require.NoError(t, err)
	require.NotNil(t, created)

	Info(context.Background(), "default logger unchanged")
	require.Len(t, logs.FilterMessage("default logger unchanged").All(), 1)
	require.NoError(t, lc.Stop(context.Background()))
}

func TestIsIgnorableSyncError(t *testing.T) {
	for _, err := range []error{syscall.EBADF, syscall.EINVAL, syscall.ENOTTY} {
		require.True(t, isIgnorableSyncError(fmt.Errorf("sync logger: %w", err)))
	}
	for _, err := range []error{nil, errors.New("write failed")} {
		require.False(t, isIgnorableSyncError(err))
	}
}
