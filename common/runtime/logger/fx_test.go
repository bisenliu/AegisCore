package logger

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxevent"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func TestNewFxEventLoggerUsesNamedDebugAndErrorLevels(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	fxLog := NewFxEventLogger(zap.New(core))

	fxLog.LogEvent(&fxevent.Run{Name: "newDependency", Kind: "provide", Runtime: time.Millisecond})
	fxLog.LogEvent(&fxevent.Invoked{FunctionName: "invokeDependency", Err: errors.New("invoke failed")})

	entries := logs.AllUntimed()
	require.Len(t, entries, 2)
	require.Equal(t, zap.DebugLevel, entries[0].Level)
	require.Equal(t, "fx", entries[0].LoggerName)
	require.Equal(t, zapcore.ErrorLevel, entries[1].Level)
	require.Equal(t, "fx", entries[1].LoggerName)
}

func TestNewFxEventLoggerDoesNotReplaceDefaultLogger(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	setDefaultLoggerForTest(t, zap.New(core))

	fxLog := NewFxEventLogger(zap.NewNop())
	fxLog.LogEvent(&fxevent.Provided{ConstructorName: "newDependency"})
	Info(context.Background(), "default logger still active")

	require.Len(t, logs.FilterMessage("default logger still active").All(), 1)
}

func TestIsIgnorableSyncError(t *testing.T) {
	for _, err := range []error{syscall.EBADF, syscall.EINVAL, syscall.ENOTTY} {
		require.True(t, isIgnorableSyncError(fmt.Errorf("sync logger: %w", err)))
	}
	for _, err := range []error{nil, errors.New("write failed")} {
		require.False(t, isIgnorableSyncError(err))
	}
}
