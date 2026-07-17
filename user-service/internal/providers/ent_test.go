package providers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/user-service/ent/enttest"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestEntSQLDebugEnabledRequiresConfigFlag(t *testing.T) {
	require.False(t, entSQLDebugEnabled(nil))
	require.False(t, entSQLDebugEnabled(&serviceconfig.Config{}))
	require.True(t, entSQLDebugEnabled(&serviceconfig.Config{Ent: serviceconfig.EntConfig{SQLDebug: true}}))
}

func TestEntObservabilityDriverLogsDebugSQLWithStableFields(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	log := logger.SQL(zap.New(core))
	ctx := contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708")
	driver := newEntObservabilityDriver(observabilityTestDriver{}, log, primaryDatabaseResource, true)
	driver.now = advancingClock(time.Unix(0, 0), 12*time.Millisecond)

	require.NoError(t, driver.Exec(ctx, "SELECT * FROM users WHERE id = $1", []any{1}, nil))

	entries := logs.FilterMessage("ent sql completed").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "00112233445566778899aabbccddeeff", fields[logger.TraceIDField])
	require.Equal(t, "0102030405060708", fields[logger.SpanIDField])
	require.Equal(t, primaryDatabaseResource, fields["db"])
	require.Equal(t, "select", fields["operation"])
	require.Equal(t, int64(12), fields["duration_ms"])
	require.Equal(t, "postgres", fields[logger.ComponentField])
	require.Equal(t, logger.SQLLoggerName, entries[0].LoggerName)
	require.Equal(t, zap.DebugLevel, entries[0].Level)
}

func TestEntObservabilityDriverLogsSlowSQLAtWarnWhenDebugDisabled(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	driver := newEntObservabilityDriver(observabilityTestDriver{}, logger.SQL(zap.New(core)), primaryDatabaseResource, false)
	driver.now = advancingClock(time.Unix(0, 0), defaultEntSlowQueryThreshold)

	require.NoError(t, driver.Query(context.Background(), "SELECT 1", nil, nil))

	entries := logs.FilterMessage("ent sql slow").All()
	require.Len(t, entries, 1)
	require.Equal(t, zap.WarnLevel, entries[0].Level)
	require.Equal(t, int64(defaultEntSlowQueryThreshold/time.Millisecond), entries[0].ContextMap()["duration_ms"])
}

func TestEntObservabilityDriverLogsSQLErrorWhenDebugDisabled(t *testing.T) {
	wantErr := errors.New("query failed")
	core, logs := observer.New(zap.DebugLevel)
	driver := newEntObservabilityDriver(observabilityTestDriver{queryErr: wantErr}, logger.SQL(zap.New(core)), primaryDatabaseResource, false)
	driver.now = advancingClock(time.Unix(0, 0), time.Millisecond)

	err := driver.Query(context.Background(), "UPDATE users SET name = $1", nil, nil)
	require.ErrorIs(t, err, wantErr)

	entries := logs.FilterMessage("ent sql failed").All()
	require.Len(t, entries, 1)
	require.Equal(t, zap.ErrorLevel, entries[0].Level)
	fields := entries[0].ContextMap()
	require.Equal(t, "update", fields["operation"])
	require.Equal(t, wantErr.Error(), fields["error"])
}

func TestNewEntDriverAlwaysObservesErrorsAndUsesConfiguredDebugLevel(t *testing.T) {
	driver, ok := newEntDriver(nil, &serviceconfig.Config{}, zap.NewNop()).(*entObservabilityDriver)
	require.True(t, ok)
	require.False(t, driver.debugEnabled)
	driver, ok = newEntDriver(nil, &serviceconfig.Config{Ent: serviceconfig.EntConfig{SQLDebug: true}}, zap.NewNop()).(*entObservabilityDriver)
	require.True(t, ok)
	require.True(t, driver.debugEnabled)
}

func TestCloseEntClientPreservesNamedError(t *testing.T) {
	userErr := errors.New("user close failed")

	err := closeEntClient("primary_db", func() error { return userErr })
	require.Error(t, err)
	require.ErrorIs(t, err, userErr)
	require.ErrorContains(t, err, "close primary_db ent client")
}

func TestCloseEntClientCallsCloser(t *testing.T) {
	closed := false

	err := closeEntClient("primary_db", func() error {
		closed = true
		return nil
	})
	require.NoError(t, err)
	require.True(t, closed)
}

func TestEntQueryObservabilityRecordsSpanAndMetrics(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_observability_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true}
	tracingProvider, recorder := newGinTestTracingProviderWithRecorder(t, cfg)
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	metrics, err := newEntQueryMetrics(metricsProvider)
	require.NoError(t, err)
	installEntQueryObservability(client, tracingProvider.Tracer("ent-test"), metrics)

	ctx, span := tracingProvider.Tracer("ent-test").Start(context.Background(), "parent")
	count, err := client.User.Query().Count(ctx)
	span.End()

	require.NoError(t, err)
	require.Zero(t, count)
	require.True(t, hasEntQuerySpan(recorder.Ended()), "spans=%v", entSpanNames(recorder.Ended()))
	latency := gatherGinMetricFamily(t, metricsProvider, entQueryLatencyMetricName)
	latencyMetric := findGinMetricByLabels(t, latency, map[string]string{
		"entity": "user",
		"query":  entQueryOperation,
		"result": entResultSuccess,
	})
	require.Equal(t, uint64(1), latencyMetric.GetHistogram().GetSampleCount())
}

func TestEntQueryObservabilityRecordsErrorMetric(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_observability_error_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true}
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	metrics, err := newEntQueryMetrics(metricsProvider)
	require.NoError(t, err)
	installEntQueryObservability(client, nil, metrics)
	require.NoError(t, client.Close())

	_, err = client.User.Query().Count(context.Background())
	require.Error(t, err)
	errorFamily := gatherGinMetricFamily(t, metricsProvider, entQueryErrorMetricName)
	errorMetric := findGinMetricByLabels(t, errorFamily, map[string]string{
		"entity": "user",
		"query":  entQueryOperation,
	})
	require.Equal(t, float64(1), errorMetric.GetCounter().GetValue())
}

func TestEntQueryObservabilityDisabledKeepsQueryResult(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_observability_disabled_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, installEntObservability(client, newProviderTestMetrics(t, false), newProviderTestDisabledTracing(t)))

	count, err := client.User.Query().Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestEntQueryObservabilityNilFallbackKeepsQueryResultForDirectConstruction(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_observability_nil_fallback_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, installEntObservability(client, nil, nil))

	count, err := client.User.Query().Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
}

func hasEntQuerySpan(spans []sdktrace.ReadOnlySpan) bool {
	for _, span := range spans {
		if span.Name() == "ent.query" {
			return true
		}
	}
	return false
}

func entSpanNames(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name())
	}
	return names
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

type observabilityTestDriver struct {
	execErr  error
	queryErr error
	txErr    error
}

func (d observabilityTestDriver) Exec(context.Context, string, any, any) error {
	return d.execErr
}

func (d observabilityTestDriver) Query(context.Context, string, any, any) error {
	return d.queryErr
}

func (d observabilityTestDriver) Tx(context.Context) (dialect.Tx, error) {
	if d.txErr != nil {
		return nil, d.txErr
	}
	return dialect.NopTx(d), nil
}

func (observabilityTestDriver) Close() error {
	return nil
}

func (observabilityTestDriver) Dialect() string {
	return dialect.Postgres
}

func advancingClock(start time.Time, elapsed time.Duration) func() time.Time {
	calls := 0
	return func() time.Time {
		calls++
		if calls%2 == 1 {
			return start
		}
		return start.Add(elapsed)
	}
}
