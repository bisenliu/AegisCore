package providers

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
