package providers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"go.opentelemetry.io/otel/trace"
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
	ctx := contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708")

	entSQLDebugLogFunc(log)(ctx, "driver.Query: query=SELECT * FROM users WHERE id = $1 args=[1]")

	entries := logs.FilterMessage("ent sql debug").All()
	if len(entries) != 1 {
		t.Fatalf("log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields[logger.TraceIDField] != "00112233445566778899aabbccddeeff" {
		t.Fatalf("trace_id = %v, want OTel trace ID", fields[logger.TraceIDField])
	}
	if fields[logger.SpanIDField] != "0102030405060708" {
		t.Fatalf("span_id = %v, want OTel span ID", fields[logger.SpanIDField])
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

func contextWithSpanContext(ctx context.Context, t *testing.T, traceIDHex string, spanIDHex string) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex(traceIDHex)
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	spanID, err := trace.SpanIDFromHex(spanIDHex)
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	return trace.ContextWithSpanContext(ctx, spanContext)
}
