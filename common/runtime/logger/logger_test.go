package logger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
)

func TestNewWritesClassifiedFiles(t *testing.T) {
	dir := t.TempDir()
	log, err := NewWithConfig(config.LogConfig{
		Level:      "debug",
		Format:     "json",
		Directory:  dir,
		Filename:   "aegiscore-test",
		Console:    false,
		MaxAgeDays: 1,
		MaxSizeMB:  1,
		MaxBackups: 1,
	})
	require.NoError(t, err, "NewWithConfig")
	ctx := contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708")
	ctx = ToContext(ctx, log)

	Debug(ctx, "debug message")
	Info(ctx, "info message", zap.String("example", "logger.Info(ctx, ...)"))
	Warn(ctx, "warn message")
	Error(ctx, "error message")
	require.NoError(t, log.Sync(), "Sync")

	date := time.Now().Format("2006-01-02")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "all"), "debug message", "info message", "warn message", "error message", `"trace_id":"00112233445566778899aabbccddeeff"`, `"span_id":"0102030405060708"`)
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "info"), "info message")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", date, "info"), "warn message", "error message")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "warning"), "warn message")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", date, "warning"), "info message", "error message")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "error"), "error message")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", date, "error"), "info message", "warn message")
	assertFileMissingOrNotContains(t, datedPath(dir, "aegiscore-test", date, "sql"), "debug message", "info message", "warn message", "error message")
}

func TestSQLLoggerWritesAllAndSQLFiles(t *testing.T) {
	dir := t.TempDir()
	log, err := NewWithConfig(config.LogConfig{
		Level:      "info",
		Format:     "json",
		Directory:  dir,
		Filename:   "aegiscore-test",
		Console:    false,
		MaxAgeDays: 1,
		MaxSizeMB:  1,
		MaxBackups: 1,
	})
	require.NoError(t, err, "NewWithConfig")

	Info(ToContext(context.Background(), SQL(log)), "ent sql debug", zap.String("statement", "SELECT 1"))
	Info(ToContext(context.Background(), log), "regular info")
	require.NoError(t, log.Sync(), "Sync")

	date := time.Now().Format("2006-01-02")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "all"), "ent sql debug", "regular info", "SELECT 1")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "sql"), "ent sql debug", "SELECT 1")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "info"), "regular info")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", date, "sql"), "regular info")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", date, "info"), "ent sql debug", "SELECT 1")
}

func TestErrorDoesNotIncludeStacktraceByDefault(t *testing.T) {
	dir := t.TempDir()
	log, err := NewWithConfig(config.LogConfig{
		Level:      "debug",
		Format:     "json",
		Directory:  dir,
		Filename:   "aegiscore-test",
		Console:    false,
		MaxAgeDays: 1,
		MaxSizeMB:  1,
		MaxBackups: 1,
	})
	require.NoError(t, err, "NewWithConfig")
	ctx := ToContext(context.Background(), log)

	Error(ctx, "error without stacktrace")
	require.NoError(t, log.Sync(), "Sync")

	path := datedPath(dir, "aegiscore-test", time.Now().Format("2006-01-02"), "error")
	assertFileContains(t, path, "error without stacktrace", `"caller"`)
	assertFileNotContains(t, path, `"stacktrace"`)
}

func TestExplicitStacktraceField(t *testing.T) {
	dir := t.TempDir()
	log, err := NewWithConfig(config.LogConfig{
		Level:      "debug",
		Format:     "json",
		Directory:  dir,
		Filename:   "aegiscore-test",
		Console:    false,
		MaxAgeDays: 1,
		MaxSizeMB:  1,
		MaxBackups: 1,
	})
	require.NoError(t, err, "NewWithConfig")
	ctx := ToContext(context.Background(), log)

	Error(ctx, "error with stacktrace", StackTrace()...)
	require.NoError(t, log.Sync(), "Sync")

	assertFileContains(t, datedPath(dir, "aegiscore-test", time.Now().Format("2006-01-02"), "error"), "error with stacktrace", `"stacktrace"`)
}

func TestWithContextAddsOTelTraceAndSpanFields(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	ctx := contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708")

	WithContext(ctx, log).Info("otel fields")

	entries := logs.FilterMessage("otel fields").All()
	require.Len(t, entries, 1, "log count")
	fields := entries[0].ContextMap()
	require.Equal(t, "00112233445566778899aabbccddeeff", fields[TraceIDField])
	require.Equal(t, "0102030405060708", fields[SpanIDField])
}

func TestWithContextOmitsTraceAndSpanFieldsWithoutValidSpan(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)

	WithContext(context.Background(), log).Info("no span fields")

	entries := logs.FilterMessage("no span fields").All()
	require.Len(t, entries, 1, "log count")
	fields := entries[0].ContextMap()
	require.NotContains(t, fields, TraceIDField)
	require.NotContains(t, fields, SpanIDField)
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
	SetDefault(log)
	t.Cleanup(func() { SetDefault(nil) })

	contextLoggerCallerProbe(context.Background())
	assertCallerFromLoggerTest(t, logs.FilterMessage("caller probe").All())
}

func assertCallerFromLoggerTest(t *testing.T, entries []observer.LoggedEntry) {
	t.Helper()
	require.Len(t, entries, 1, "log count")
	caller := entries[0].Caller
	require.Falsef(t, strings.HasSuffix(caller.File, "common/runtime/logger/context.go"), "caller = %s:%d, want test call site", caller.File, caller.Line)
	require.Truef(t, strings.HasSuffix(caller.File, "common/runtime/logger/logger_test.go"), "caller file = %s, want logger_test.go", caller.File)
}

func contextLoggerCallerProbe(ctx context.Context) {
	Info(ctx, "caller probe")
}

func TestDefaultLoggerConcurrentAccess(_ *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
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
	SetDefault(nil)
}

func TestDailyWriterRotatesWhenDateChanges(t *testing.T) {
	dir := t.TempDir()
	writer := newDailyLumberjackWriteSyncer(filepath.Join(dir, "aegiscore-test.all.log"), config.LogConfig{MaxAgeDays: 1, MaxSizeMB: 1, MaxBackups: 1})
	daily, ok := writer.(*dailyLumberjackWriteSyncer)
	require.Truef(t, ok, "writer type = %T, want *dailyLumberjackWriteSyncer", writer)
	now := time.Date(2026, 5, 29, 8, 0, 0, 0, time.Local)
	daily.newClock = func() time.Time { return now }
	_, err := daily.Write([]byte("first day\n"))
	require.NoError(t, err, "Write first day")
	now = now.AddDate(0, 0, 1)
	_, err = daily.Write([]byte("second day\n"))
	require.NoError(t, err, "Write second day")
	require.NoError(t, daily.Sync(), "Sync")
	assertFileContains(t, datedPath(dir, "aegiscore-test", "2026-05-29", "all"), "first day")
	assertFileContains(t, datedPath(dir, "aegiscore-test", "2026-05-30", "all"), "second day")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", "2026-05-29", "all"), "second day")
}

func TestDailyWriterSyncDoesNotPreventFurtherWrites(t *testing.T) {
	dir := t.TempDir()
	writer := newDailyLumberjackWriteSyncer(filepath.Join(dir, "aegiscore-test.all.log"), config.LogConfig{MaxAgeDays: 1, MaxSizeMB: 1, MaxBackups: 1})
	daily, ok := writer.(*dailyLumberjackWriteSyncer)
	require.Truef(t, ok, "writer type = %T, want *dailyLumberjackWriteSyncer", writer)
	now := time.Date(2026, 5, 29, 8, 0, 0, 0, time.Local)
	daily.newClock = func() time.Time { return now }

	_, err := daily.Write([]byte("before sync\n"))
	require.NoError(t, err, "Write before sync")
	require.NoError(t, daily.Sync(), "Sync")
	_, err = daily.Write([]byte("after sync\n"))
	require.NoError(t, err, "Write after sync")

	assertFileContains(t, datedPath(dir, "aegiscore-test", "2026-05-29", "all"), "before sync", "after sync")
}

func TestDailyWriterAppliesLumberjackConfig(t *testing.T) {
	dir := t.TempDir()
	writer := newDailyLumberjackWriteSyncer(filepath.Join(dir, "aegiscore-test.all.log"), config.LogConfig{MaxAgeDays: 5, MaxSizeMB: 6, MaxBackups: 7})
	daily := writer.(*dailyLumberjackWriteSyncer)
	daily.mu.Lock()
	defer daily.mu.Unlock()
	require.Equal(t, 6, daily.logger.MaxSize)
	require.Equal(t, 7, daily.logger.MaxBackups)
	require.Equal(t, 5, daily.logger.MaxAge)
	require.Equal(t, datedPath(dir, "aegiscore-test", daily.date, "all"), daily.logger.Filename)
}

func assertFileContains(t *testing.T, path string, wants ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoErrorf(t, err, "ReadFile(%s)", path)
	text := string(content)
	for _, want := range wants {
		require.Containsf(t, text, want, "%s content", path)
	}
}

func assertFileNotContains(t *testing.T, path string, wants ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoErrorf(t, err, "ReadFile(%s)", path)
	text := string(content)
	for _, want := range wants {
		require.NotContainsf(t, text, want, "%s content", path)
	}
}

func assertFileMissingOrNotContains(t *testing.T, path string, wants ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	require.NoErrorf(t, err, "ReadFile(%s)", path)
	text := string(content)
	for _, want := range wants {
		require.NotContainsf(t, text, want, "file %s content", path)
	}
}

func datedPath(dir string, prefix string, date string, level string) string {
	return filepath.Join(dir, prefix+"."+date+"."+level+".log")
}

func contextWithSpanContext(ctx context.Context, t *testing.T, traceIDHex string, spanIDHex string) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex(traceIDHex)
	require.NoError(t, err, "TraceIDFromHex")
	spanID, err := trace.SpanIDFromHex(spanIDHex)
	require.NoError(t, err, "SpanIDFromHex")
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	return trace.ContextWithSpanContext(ctx, spanContext)
}
