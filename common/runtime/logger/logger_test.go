package logger

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
)

func TestNewLoggerSplitsLowAndWarningLevels(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	log, err := newLogger(
		config.LogConfig{Level: "debug", Format: "json"},
		"user-service",
		"test",
		zapcore.AddSync(&stdout),
		zapcore.AddSync(&stderr),
	)
	require.NoError(t, err)

	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message", zap.Error(context.Canceled))

	require.Contains(t, stdout.String(), "debug message")
	require.Contains(t, stdout.String(), "info message")
	require.NotContains(t, stdout.String(), "warn message")
	require.NotContains(t, stdout.String(), "error message")
	require.Contains(t, stderr.String(), "warn message")
	require.Contains(t, stderr.String(), "error message")
	require.NotContains(t, stderr.String(), "info message")
	require.Contains(t, stderr.String(), `"error":"context canceled"`)
	for _, output := range []string{stdout.String(), stderr.String()} {
		require.Contains(t, output, `"logger":"application"`)
		require.Contains(t, output, `"service":"user-service"`)
		require.Contains(t, output, `"env":"test"`)
	}
}

func TestNewLoggerHonorsConfiguredLevelAndConsoleFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	log, err := newLogger(
		config.LogConfig{Level: "error", Format: "console"},
		"",
		"",
		zapcore.AddSync(&stdout),
		zapcore.AddSync(&stderr),
	)
	require.NoError(t, err)

	log.Info("ignored")
	log.Error("visible")

	require.Empty(t, stdout.String())
	require.NotContains(t, stderr.String(), "ignored")
	require.Contains(t, stderr.String(), "visible")
}

func TestNewDoesNotReplaceDefaultLogger(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	setDefaultLoggerForTest(t, zap.New(core))

	created, err := New(&config.Config{App: config.AppConfig{Name: "user-service", Environment: "test"}})
	require.NoError(t, err)
	require.NotNil(t, created)

	Info(context.Background(), "default logger unchanged")
	require.Len(t, logs.FilterMessage("default logger unchanged").All(), 1)
}

func TestNewWithConfigDoesNotReplaceDefaultLogger(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	setDefaultLoggerForTest(t, zap.New(core))

	created, err := NewWithConfig(config.LogConfig{Level: "debug", Format: "json"})
	require.NoError(t, err)
	require.NotNil(t, created)

	Info(context.Background(), "default logger unchanged")
	require.Len(t, logs.FilterMessage("default logger unchanged").All(), 1)
}

func TestNamedComponentResetsLoggerNameAndAddsComponent(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	base := zap.New(core).Named("application").With(zap.String(ServiceField, "user-service"))

	SQL(base).Debug("sql event")

	entries := logs.FilterMessage("sql event").All()
	require.Len(t, entries, 1)
	require.Equal(t, SQLLoggerName, entries[0].LoggerName)
	require.Equal(t, "postgres", entries[0].ContextMap()[ComponentField])
	require.Equal(t, "user-service", entries[0].ContextMap()[ServiceField])
}

func TestErrorDoesNotIncludeStacktraceByDefault(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core, zap.AddCaller())

	Error(ToContext(context.Background(), log), "error without stacktrace")

	entries := logs.FilterMessage("error without stacktrace").All()
	require.Len(t, entries, 1)
	require.Empty(t, entries[0].Stack)
}

func TestExplicitStacktraceField(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)

	Error(ToContext(context.Background(), log), "error with stacktrace", StackTrace()...)

	entries := logs.FilterMessage("error with stacktrace").All()
	require.Len(t, entries, 1)
	require.NotEmpty(t, entries[0].ContextMap()["stacktrace"])
}

func TestWithContextAddsOTelTraceAndSpanFields(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	ctx := contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708")

	WithContext(ctx, log).Info("otel fields")

	entries := logs.FilterMessage("otel fields").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "00112233445566778899aabbccddeeff", fields[TraceIDField])
	require.Equal(t, "0102030405060708", fields[SpanIDField])
}

func TestWithRequestIDStoresOnlyNonEmptyValue(t *testing.T) {
	ctx := WithRequestID(context.Background(), "request-123")
	requestID, ok := RequestIDFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "request-123", requestID)

	emptyCtx := WithRequestID(context.Background(), "")
	_, ok = RequestIDFromContext(emptyCtx)
	require.False(t, ok)
}

func TestWithContextAddsRequestIDWithoutValidSpan(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	ctx := WithRequestID(context.Background(), "request-123")

	WithContext(ctx, log).Info("request id field")

	entries := logs.FilterMessage("request id field").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "request-123", fields[RequestIDField])
	require.NotContains(t, fields, TraceIDField)
	require.NotContains(t, fields, SpanIDField)
}

func TestFromContextAddsTraceSpanAndRequestID(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	ctx := contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708")
	ctx = WithRequestID(ctx, "request-123")
	ctx = ToContext(ctx, log)

	Info(ctx, "all correlation fields")

	entries := logs.FilterMessage("all correlation fields").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "00112233445566778899aabbccddeeff", fields[TraceIDField])
	require.Equal(t, "0102030405060708", fields[SpanIDField])
	require.Equal(t, "request-123", fields[RequestIDField])
}

func TestWithContextOmitsTraceAndSpanFieldsWithoutValidSpan(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)

	WithContext(context.Background(), log).Info("no span fields")

	entries := logs.FilterMessage("no span fields").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.NotContains(t, fields, TraceIDField)
	require.NotContains(t, fields, SpanIDField)
	require.NotContains(t, fields, RequestIDField)
}

func TestContextLoggerReportsCallerFromCallSite(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core, zap.AddCaller())
	ctx := ToContext(context.Background(), log)

	contextLoggerCallerProbe(ctx)
	assertCallerFromLoggerTest(t, logs.FilterMessage("caller probe").All())
}

func TestDefaultLoggerReportsCallerFromCallSite(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core, zap.AddCaller())
	setDefaultLoggerForTest(t, log)

	contextLoggerCallerProbe(context.Background())
	assertCallerFromLoggerTest(t, logs.FilterMessage("caller probe").All())
}

func assertCallerFromLoggerTest(t *testing.T, entries []observer.LoggedEntry) {
	t.Helper()
	require.Len(t, entries, 1)
	caller := entries[0].Caller
	require.Falsef(t, strings.HasSuffix(caller.File, "common/runtime/logger/context.go"), "caller = %s:%d, want test call site", caller.File, caller.Line)
	require.Truef(t, strings.HasSuffix(caller.File, "common/runtime/logger/logger_test.go"), "caller file = %s, want logger_test.go", caller.File)
}

func contextLoggerCallerProbe(ctx context.Context) {
	Info(ctx, "caller probe")
}

func TestDefaultLoggerConcurrentAccess(t *testing.T) {
	restoreDefaultLoggerAfterTest(t)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			SetDefault(zap.NewNop())
		}()
		go func() {
			defer wg.Done()
			Info(context.Background(), "concurrent logger access")
		}()
	}
	wg.Wait()
}

func setDefaultLoggerForTest(t *testing.T, log *zap.Logger) {
	t.Helper()
	restoreDefaultLoggerAfterTest(t)
	SetDefault(log)
}

func restoreDefaultLoggerAfterTest(t *testing.T) {
	t.Helper()
	previous := getDefault()
	t.Cleanup(func() { SetDefault(previous) })
}

func contextWithSpanContext(ctx context.Context, t *testing.T, traceIDHex string, spanIDHex string) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex(traceIDHex)
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex(spanIDHex)
	require.NoError(t, err)
	return trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	}))
}
