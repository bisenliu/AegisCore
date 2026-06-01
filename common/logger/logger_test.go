package logger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	"go.uber.org/zap"
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
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	ctx := WithTraceID(context.Background(), "trace-123")
	ctx = ToContext(ctx, log)

	Debug(ctx, "debug message")
	Info(ctx, "info message", zap.String("example", "logger.Info(ctx, ...)"))
	Warn(ctx, "warn message")
	Error(ctx, "error message")
	if err := log.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	date := time.Now().Format("2006-01-02")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "all"), "debug message", "info message", "warn message", "error message", `"trace-id":"trace-123"`)
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "info"), "info message")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", date, "info"), "warn message", "error message")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "warning"), "warn message")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", date, "warning"), "info message", "error message")
	assertFileContains(t, datedPath(dir, "aegiscore-test", date, "error"), "error message")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", date, "error"), "info message", "warn message")
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
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	ctx := ToContext(context.Background(), log)

	Error(ctx, "error without stacktrace")
	if err := log.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

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
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	ctx := ToContext(context.Background(), log)

	Error(ctx, "error with stacktrace", StackTrace()...)
	if err := log.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	assertFileContains(t, datedPath(dir, "aegiscore-test", time.Now().Format("2006-01-02"), "error"), "error with stacktrace", `"stacktrace"`)
}

func TestTraceIDHelpers(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-abc")
	if got := TraceIDFromContext(ctx); got != "trace-abc" {
		t.Fatalf("TraceIDFromContext = %q, want trace-abc", got)
	}
	if got := TraceIDFromContext(context.Background()); got != "" {
		t.Fatalf("TraceIDFromContext empty = %q, want empty", got)
	}
}

func TestDailyWriterRotatesWhenDateChanges(t *testing.T) {
	dir := t.TempDir()
	writer := newDailyLumberjackWriteSyncer(filepath.Join(dir, "aegiscore-test.all.log"), config.LogConfig{MaxAgeDays: 1, MaxSizeMB: 1, MaxBackups: 1})
	daily, ok := writer.(*dailyLumberjackWriteSyncer)
	if !ok {
		t.Fatalf("writer type = %T, want *dailyLumberjackWriteSyncer", writer)
	}
	now := time.Date(2026, 5, 29, 8, 0, 0, 0, time.Local)
	daily.newClock = func() time.Time { return now }
	if _, err := daily.Write([]byte("first day\n")); err != nil {
		t.Fatalf("Write first day: %v", err)
	}
	now = now.AddDate(0, 0, 1)
	if _, err := daily.Write([]byte("second day\n")); err != nil {
		t.Fatalf("Write second day: %v", err)
	}
	if err := daily.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	assertFileContains(t, datedPath(dir, "aegiscore-test", "2026-05-29", "all"), "first day")
	assertFileContains(t, datedPath(dir, "aegiscore-test", "2026-05-30", "all"), "second day")
	assertFileNotContains(t, datedPath(dir, "aegiscore-test", "2026-05-29", "all"), "second day")
}

func TestDailyWriterAppliesLumberjackConfig(t *testing.T) {
	dir := t.TempDir()
	writer := newDailyLumberjackWriteSyncer(filepath.Join(dir, "aegiscore-test.all.log"), config.LogConfig{MaxAgeDays: 5, MaxSizeMB: 6, MaxBackups: 7})
	daily := writer.(*dailyLumberjackWriteSyncer)
	daily.mu.Lock()
	defer daily.mu.Unlock()
	if daily.logger.MaxSize != 6 {
		t.Fatalf("MaxSize = %d, want 6", daily.logger.MaxSize)
	}
	if daily.logger.MaxBackups != 7 {
		t.Fatalf("MaxBackups = %d, want 7", daily.logger.MaxBackups)
	}
	if daily.logger.MaxAge != 5 {
		t.Fatalf("MaxAge = %d, want 5", daily.logger.MaxAge)
	}
	if daily.logger.Filename != datedPath(dir, "aegiscore-test", daily.date, "all") {
		t.Fatalf("Filename = %q, want dated file", daily.logger.Filename)
	}
}

func assertFileContains(t *testing.T, path string, wants ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	text := string(content)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("%s does not contain %q; content: %s", path, want, text)
		}
	}
}

func assertFileNotContains(t *testing.T, path string, wants ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	text := string(content)
	for _, want := range wants {
		if strings.Contains(text, want) {
			t.Fatalf("%s contains %q; content: %s", path, want, text)
		}
	}
}

func datedPath(dir string, prefix string, date string, level string) string {
	return filepath.Join(dir, prefix+"."+date+"."+level+".log")
}
