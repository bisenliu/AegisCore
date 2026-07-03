package providers

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
)

func TestEntSQLDebugEnabledRequiresConfigFlag(t *testing.T) {
	require.False(t, entSQLDebugEnabled(nil))
	require.False(t, entSQLDebugEnabled(&config.Config{}))
	require.True(t, entSQLDebugEnabled(&config.Config{Ent: config.EntConfig{SQLDebug: true}}))
}

func TestEntSQLDebugLogFuncWritesSQLDiagnosticLog(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core).Named(logger.SQLLoggerName)
	ctx := contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708")

	entSQLDebugLogFunc(log)(ctx, "driver.Query: query=SELECT * FROM users WHERE id = $1 args=[1]")

	entries := logs.FilterMessage("ent sql debug").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "00112233445566778899aabbccddeeff", fields[logger.TraceIDField])
	require.Equal(t, "0102030405060708", fields[logger.SpanIDField])
	require.Equal(t, "driver.Query: query=SELECT * FROM users WHERE id = $1 args=[1]", fields["statement"])
	require.Equal(t, logger.SQLLoggerName, entries[0].LoggerName)
}

func TestNewEntDriverUsesDebugDriverOnlyWhenConfigured(t *testing.T) {
	_, ok := newEntDriver(nil, &config.Config{}, zap.NewNop()).(*dialect.DebugDriver)
	require.False(t, ok)
	_, ok = newEntDriver(nil, &config.Config{Ent: config.EntConfig{SQLDebug: true}}, zap.NewNop()).(*dialect.DebugDriver)
	require.True(t, ok)
}

func TestCloseEntClientPreservesNamedError(t *testing.T) {
	userErr := errors.New("user close failed")

	err := closeEntClient("user_db", func() error { return userErr })
	require.Error(t, err)
	require.ErrorIs(t, err, userErr)
	require.ErrorContains(t, err, "close user_db ent client")
}

func TestCloseEntClientCallsCloser(t *testing.T) {
	closed := false

	err := closeEntClient("user_db", func() error {
		closed = true
		return nil
	})
	require.NoError(t, err)
	require.True(t, closed)
}

func contextWithSpanContext(ctx context.Context, t *testing.T, traceIDHex string, spanIDHex string) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex(traceIDHex)
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex(spanIDHex)
	require.NoError(t, err)
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	return trace.ContextWithSpanContext(ctx, spanContext)
}
