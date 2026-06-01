package logger

import (
	"context"
	"fmt"
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

	assertFileContains(t, filepath.Join(dir, "aegiscore-test.all.log"), "debug message", "info message", "warn message", "error message", `"trace-id":"trace-123"`)
	assertFileContains(t, filepath.Join(dir, "aegiscore-test.info.log"), "info message")
	assertFileNotContains(t, filepath.Join(dir, "aegiscore-test.info.log"), "warn message", "error message")
	assertFileContains(t, filepath.Join(dir, "aegiscore-test.warning.log"), "warn message")
	assertFileNotContains(t, filepath.Join(dir, "aegiscore-test.warning.log"), "info message", "error message")
	assertFileContains(t, filepath.Join(dir, "aegiscore-test.error.log"), "error message")
	assertFileNotContains(t, filepath.Join(dir, "aegiscore-test.error.log"), "info message", "warn message")
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

	assertFileContains(t, filepath.Join(dir, "aegiscore-test.error.log"), "error without stacktrace", `"caller"`)
	assertFileNotContains(t, filepath.Join(dir, "aegiscore-test.error.log"), `"stacktrace"`)
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

	assertFileContains(t, filepath.Join(dir, "aegiscore-test.error.log"), "error with stacktrace", `"stacktrace"`)
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
	assertFileContains(t, filepath.Join(dir, "aegiscore-test-2026-05-29.all.log"), "first day")
	assertFileContains(t, filepath.Join(dir, "aegiscore-test.all.log"), "second day")
}

func TestDailyWriterDoesNotOverwriteExistingArchive(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "aegiscore-test-2026-05-29.all.log")
	if err := os.WriteFile(archivePath, []byte("existing archive\n"), 0o644); err != nil {
		t.Fatalf("WriteFile archive: %v", err)
	}
	writer := newDailyLumberjackWriteSyncer(filepath.Join(dir, "aegiscore-test.all.log"), config.LogConfig{MaxAgeDays: 7, MaxSizeMB: 1, MaxBackups: 10})
	daily := writer.(*dailyLumberjackWriteSyncer)
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

	assertFileContains(t, archivePath, "existing archive")
	assertFileNotContains(t, archivePath, "first day")
	assertFileContains(t, filepath.Join(dir, "aegiscore-test-2026-05-29.1.all.log"), "first day")
}

func TestDailyWriterCleansArchivesByAgeAndCount(t *testing.T) {
	dir := t.TempDir()
	writer := newDailyLumberjackWriteSyncer(filepath.Join(dir, "aegiscore-test.all.log"), config.LogConfig{MaxAgeDays: 2, MaxSizeMB: 1, MaxBackups: 1})
	daily := writer.(*dailyLumberjackWriteSyncer)
	archives := []string{
		"aegiscore-test-2026-05-27.all.log",
		"aegiscore-test-2026-05-28.all.log",
		"aegiscore-test-2026-05-29.all.log",
	}
	for i, name := range archives {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(fmt.Sprintf("archive %d\n", i)), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	daily.newClock = func() time.Time { return time.Date(2026, 5, 30, 8, 0, 0, 0, time.Local) }
	daily.mu.Lock()
	daily.cleanupArchivesLocked()
	daily.mu.Unlock()

	assertFileDoesNotExist(t, filepath.Join(dir, "aegiscore-test-2026-05-27.all.log"))
	assertFileDoesNotExist(t, filepath.Join(dir, "aegiscore-test-2026-05-28.all.log"))
	assertFileContains(t, filepath.Join(dir, "aegiscore-test-2026-05-29.all.log"), "archive 2")
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

func assertFileDoesNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists, want absent", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("Stat(%s): %v", path, err)
	}
}
