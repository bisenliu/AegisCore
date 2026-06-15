package providers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
)

func TestEntSQLDebugEnabledRequiresConfigFlag(t *testing.T) {
	if entSQLDebugEnabled(nil) {
		t.Fatal("entSQLDebugEnabled(nil) = true, want false")
	}
	if entSQLDebugEnabled(&config.Config{}) {
		t.Fatal("entSQLDebugEnabled(default) = true, want false")
	}
	if !entSQLDebugEnabled(&config.Config{Ent: config.EntConfig{SQLDebug: true}}) {
		t.Fatal("entSQLDebugEnabled(sql_debug) = false, want true")
	}
}

func TestEntSQLDebugLogFuncWritesSQLDiagnosticLog(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core).Named(logger.SQLLoggerName)
	ctx := logger.WithTraceID(context.Background(), "trace-sql-test")

	entSQLDebugLogFunc(log)(ctx, "driver.Query: query=SELECT * FROM users WHERE id = $1 args=[1]")

	entries := logs.FilterMessage("ent sql debug").All()
	if len(entries) != 1 {
		t.Fatalf("log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields[logger.TraceIDField] != "trace-sql-test" {
		t.Fatalf("trace_id = %v, want trace-sql-test", fields[logger.TraceIDField])
	}
	if fields["statement"] != "driver.Query: query=SELECT * FROM users WHERE id = $1 args=[1]" {
		t.Fatalf("statement = %v", fields["statement"])
	}
	if entries[0].LoggerName != logger.SQLLoggerName {
		t.Fatalf("logger name = %q, want %q", entries[0].LoggerName, logger.SQLLoggerName)
	}
}

func TestNewEntDriverUsesDebugDriverOnlyWhenConfigured(t *testing.T) {
	if _, ok := newEntDriver(nil, &config.Config{}, zap.NewNop()).(*dialect.DebugDriver); ok {
		t.Fatal("newEntDriver without sql_debug returned debug driver")
	}
	if _, ok := newEntDriver(nil, &config.Config{Ent: config.EntConfig{SQLDebug: true}}, zap.NewNop()).(*dialect.DebugDriver); !ok {
		t.Fatal("newEntDriver with sql_debug did not return debug driver")
	}
}

func TestCloseEntClientPreservesNamedError(t *testing.T) {
	userErr := errors.New("user close failed")

	err := closeEntClient("user_db", func() error { return userErr })
	if err == nil {
		t.Fatal("closeEntClient error = nil")
	}
	if !errors.Is(err, userErr) {
		t.Fatalf("closeEntClient error = %v, want user close error", err)
	}
	if !strings.Contains(err.Error(), "close user_db ent client") {
		t.Fatalf("closeEntClient error = %q, want user_db context", err.Error())
	}
}

func TestCloseEntClientCallsCloser(t *testing.T) {
	closed := false

	err := closeEntClient("user_db", func() error {
		closed = true
		return nil
	})
	if err != nil {
		t.Fatalf("closeEntClient: %v", err)
	}
	if !closed {
		t.Fatal("client close was not called")
	}
}
