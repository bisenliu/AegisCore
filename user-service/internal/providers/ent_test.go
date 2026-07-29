package providers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/logger"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	"github.com/aegiscore/user-service/internal/persistence/ent/enttest"
)

func TestEntSQLLogPluginWrapsDriverWhenEnabled(t *testing.T) {
	plugin := &entSQLLogPlugin{log: zap.NewNop(), db: primaryDatabaseResource}

	driver, err := plugin.WrapEntDriver(observabilityTestDriver{})

	require.NoError(t, err)
	_, ok := driver.(*entSQLLogDriver)
	require.True(t, ok)
}

func TestEntSQLLogPluginLogsDebugSQLWithStableFields(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	log := logger.SQL(zap.New(core))
	ctx := contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708")
	plugin := &entSQLLogPlugin{
		log:          log,
		db:           primaryDatabaseResource,
		debugEnabled: true,
		now:          advancingClock(time.Unix(0, 0), 12*time.Millisecond),
	}
	driver, err := plugin.WrapEntDriver(observabilityTestDriver{})
	require.NoError(t, err)

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

func TestEntSQLLogPluginLogsSlowSQLAtWarn(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	plugin := &entSQLLogPlugin{
		log: logger.SQL(zap.New(core)),
		db:  primaryDatabaseResource,
		now: advancingClock(time.Unix(0, 0), serviceconfig.DefaultEntSlowQueryThreshold),
	}
	driver, err := plugin.WrapEntDriver(observabilityTestDriver{})
	require.NoError(t, err)

	require.NoError(t, driver.Query(context.Background(), "SELECT 1", nil, nil))

	entries := logs.FilterMessage("ent sql slow").All()
	require.Len(t, entries, 1)
	require.Equal(t, zap.WarnLevel, entries[0].Level)
	require.Equal(t, int64(serviceconfig.DefaultEntSlowQueryThreshold/time.Millisecond), entries[0].ContextMap()["duration_ms"])
}

func TestEntSQLLogPluginLogsSQLError(t *testing.T) {
	wantErr := errors.New("query failed")
	core, logs := observer.New(zap.DebugLevel)
	plugin := &entSQLLogPlugin{
		log: logger.SQL(zap.New(core)),
		db:  primaryDatabaseResource,
		now: advancingClock(time.Unix(0, 0), time.Millisecond),
	}
	driver, err := plugin.WrapEntDriver(observabilityTestDriver{queryErr: wantErr})
	require.NoError(t, err)

	err = driver.Query(context.Background(), "UPDATE users SET name = $1", nil, nil)
	require.ErrorIs(t, err, wantErr)

	entries := logs.FilterMessage("ent sql failed").All()
	require.Len(t, entries, 1)
	require.Equal(t, zap.ErrorLevel, entries[0].Level)
	fields := entries[0].ContextMap()
	require.Equal(t, "update", fields["operation"])
	require.Equal(t, wantErr.Error(), fields["error"])
}

func TestNewEntDriverDoesNotWrapSQLLogByDefault(t *testing.T) {
	driver := newEntDriver(nil)
	_, wrapped := driver.(*entSQLLogDriver)
	require.False(t, wrapped)
	_, nonClosing := driver.(nonClosingEntDriver)
	require.True(t, nonClosing)
}

func TestNewEntPluginsDefaultEnablesTracingOnly(t *testing.T) {
	settings := serviceconfig.EntSettings{Plugins: serviceconfig.EntPluginsConfig{
		Tracing: serviceconfig.EntTracingPluginConfig{Enabled: true},
	}}

	plugins, err := newEntPlugins(settings, zap.NewNop(), newProviderTestMetrics(t, true), newProviderTestTracing(t))

	require.NoError(t, err)
	require.Empty(t, plugins.driverPlugins)
	require.Len(t, plugins.clientPlugins, 1)
	_, ok := plugins.clientPlugins[0].(entTracingPlugin)
	require.True(t, ok)
}

func TestNewEntPluginsEnablesSQLLogWhenConfigured(t *testing.T) {
	settings := serviceconfig.EntSettings{Plugins: serviceconfig.EntPluginsConfig{
		SQLLog: serviceconfig.EntSQLLogPluginConfig{Enabled: true, Debug: true, SlowThreshold: time.Second},
	}}

	plugins, err := newEntPlugins(settings, zap.NewNop(), nil, nil)

	require.NoError(t, err)
	require.Len(t, plugins.driverPlugins, 1)
	plugin, ok := plugins.driverPlugins[0].(*entSQLLogPlugin)
	require.True(t, ok)
	require.True(t, plugin.debugEnabled)
	require.Equal(t, time.Second, plugin.slowThreshold)
}

func TestNewEntPluginsEnablesMetricsWhenConfigured(t *testing.T) {
	settings := serviceconfig.EntSettings{Plugins: serviceconfig.EntPluginsConfig{
		Metrics: serviceconfig.EntMetricsPluginConfig{Enabled: true},
	}}
	metricsProvider := newProviderTestMetrics(t, true)

	plugins, err := newEntPlugins(settings, zap.NewNop(), metricsProvider, nil)

	require.NoError(t, err)
	require.Len(t, plugins.clientPlugins, 1)
	_, ok := plugins.clientPlugins[0].(entMetricsPlugin)
	require.True(t, ok)
}

func TestNewEntPluginsReturnsMetricsRegistrationError(t *testing.T) {
	metricsProvider := newProviderTestMetrics(t, true)
	require.NoError(t, metricsProvider.Register(prometheus.NewCounter(prometheus.CounterOpts{
		Name: entQueryLatencyMetricName,
		Help: "conflicting collector for constructor rollback test.",
	})))
	settings := serviceconfig.EntSettings{Plugins: serviceconfig.EntPluginsConfig{
		Metrics: serviceconfig.EntMetricsPluginConfig{Enabled: true},
	}}

	plugins, err := newEntPlugins(settings, zap.NewNop(), metricsProvider, nil)

	require.Empty(t, plugins.clientPlugins)
	require.ErrorContains(t, err, "register ent query latency metrics")
}

func TestNewEntClientAppliesDriverPluginsBeforeClientPlugins(t *testing.T) {
	order := make([]string, 0, 2)
	plugins := entPluginSet{
		driverPlugins: []entDriverPlugin{recordingDriverPlugin{order: &order}},
		clientPlugins: []entClientPlugin{recordingClientPlugin{order: &order}},
	}

	client, err := newEntClient(nil, plugins)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.Equal(t, []string{"driver", "client"}, order)
}

func TestNewEntClientClosesClientWhenClientPluginFails(t *testing.T) {
	wantErr := errors.New("install failed")
	tracking := &trackingDriver{}
	plugins := entPluginSet{
		driverPlugins: []entDriverPlugin{replaceDriverPlugin{driver: tracking}},
		clientPlugins: []entClientPlugin{failingClientPlugin{err: wantErr}},
	}

	client, err := newEntClient(nil, plugins)

	require.Nil(t, client)
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, tracking.closes)
}

func TestEntTracingPluginRecordsQuerySpan(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_tracing_query_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cfg := ginTestConfig()
	tracingProvider, collector := newGinTestTracingProviderWithRecorder(t, cfg)
	require.NoError(t, entTracingPlugin{tracer: tracingProvider.Tracer("ent-test")}.InstallEntClientPlugin(client))

	ctx, parentSpan := tracingProvider.Tracer("ent-test").Start(context.Background(), "parent")
	count, err := client.User.Query().Count(ctx)
	parentSpan.End()

	require.NoError(t, err)
	require.Zero(t, count)
	span := findEntSpan(t, exportedSpans(t, tracingProvider, collector), "ent.query")
	require.Equal(t, "user", spanAttributeValue(span, "ent.entity"))
	require.Equal(t, entQueryOperation, spanAttributeValue(span, "ent.operation"))
}

func TestEntTracingPluginRecordsMutationSpan(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_tracing_mutation_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cfg := ginTestConfig()
	tracingProvider, collector := newGinTestTracingProviderWithRecorder(t, cfg)
	require.NoError(t, entTracingPlugin{tracer: tracingProvider.Tracer("ent-test")}.InstallEntClientPlugin(client))

	ctx, parentSpan := tracingProvider.Tracer("ent-test").Start(context.Background(), "parent")
	_, err := client.User.Create().
		SetUserID(runtimeid.MustNewUUID()).
		SetNickname("tester").
		SetUsername("tester").
		SetPasswordHash("hash").
		Save(ctx)
	parentSpan.End()

	require.NoError(t, err)
	span := findEntSpan(t, exportedSpans(t, tracingProvider, collector), "ent.mutation")
	require.Equal(t, "user", spanAttributeValue(span, "ent.entity"))
	require.Equal(t, "create", spanAttributeValue(span, "ent.operation"))
}

func TestEntTracingPluginRecordsErrorStatus(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_tracing_error_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	cfg := ginTestConfig()
	tracingProvider, collector := newGinTestTracingProviderWithRecorder(t, cfg)
	require.NoError(t, entTracingPlugin{tracer: tracingProvider.Tracer("ent-test")}.InstallEntClientPlugin(client))
	require.NoError(t, client.Close())

	_, err := client.User.Query().Count(context.Background())

	require.Error(t, err)
	span := findEntSpan(t, exportedSpans(t, tracingProvider, collector), "ent.query")
	require.Equal(t, tracepb.Status_STATUS_CODE_ERROR, span.GetStatus().GetCode())
	require.NotEmpty(t, span.GetEvents())
}

func TestEntTracingDisabledDoesNotInstallSpan(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_tracing_disabled_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cfg := ginTestConfig()
	tracingProvider, collector := newGinTestTracingProviderWithRecorder(t, cfg)

	ctx, span := tracingProvider.Tracer("ent-test").Start(context.Background(), "parent")
	_, err := client.User.Query().Count(ctx)
	span.End()

	require.NoError(t, err)
	require.False(t, hasEntSpan(exportedSpans(t, tracingProvider, collector)))
}

func TestEntMetricsPluginRecordsQueryLatency(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_metrics_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true}
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	metrics, err := newEntQueryMetrics(metricsProvider)
	require.NoError(t, err)
	require.NoError(t, entMetricsPlugin{metrics: metrics}.InstallEntClientPlugin(client))

	count, err := client.User.Query().Count(context.Background())

	require.NoError(t, err)
	require.Zero(t, count)
	latency := gatherGinMetricFamily(t, metricsProvider, entQueryLatencyMetricName)
	latencyMetric := findGinMetricByLabels(t, latency, map[string]string{
		"entity": "user",
		"query":  entQueryOperation,
		"result": entResultSuccess,
	})
	require.Equal(t, uint64(1), latencyMetric.GetHistogram().GetSampleCount())
}

func TestEntMetricsPluginRecordsQueryError(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:ent_metrics_error_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true}
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	metrics, err := newEntQueryMetrics(metricsProvider)
	require.NoError(t, err)
	require.NoError(t, entMetricsPlugin{metrics: metrics}.InstallEntClientPlugin(client))
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

func TestEntMetricsDisabledDoesNotRegisterCollectors(t *testing.T) {
	cfg := ginTestConfig()
	cfg.Observability.Metrics = config.MetricsConfig{Enabled: true}
	metricsProvider := newGinTestMetricsProvider(t, cfg)
	plugins, err := newEntPlugins(serviceconfig.EntSettings{}, zap.NewNop(), metricsProvider, nil)

	require.NoError(t, err)
	require.Empty(t, plugins.clientPlugins)
	assertGinMetricFamilyMissing(t, metricsProvider, entQueryLatencyMetricName)
}

func TestNonClosingEntDriverCloseDoesNotCloseUnderlyingDB(t *testing.T) {
	drv := registerProviderTestSQLDriver(t)
	db, err := sql.Open(drv.name, "postgres://aegiscore:secret@127.0.0.1/aegiscore_user")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	driver := nonClosingEntDriver{Driver: observabilityTestDriver{}}

	require.NoError(t, driver.Close())
	require.NoError(t, db.PingContext(context.Background()))
	require.Equal(t, int64(0), drv.closes.Load())
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

func hasEntSpan(spans []*tracepb.Span) bool {
	for _, span := range spans {
		if span.GetName() == "ent.query" || span.GetName() == "ent.mutation" {
			return true
		}
	}
	return false
}

func findEntSpan(t *testing.T, spans []*tracepb.Span, name string) *tracepb.Span {
	t.Helper()
	for _, span := range spans {
		if span.GetName() == name {
			return span
		}
	}
	require.Failf(t, "span not found", "span %q not found in %v", name, entSpanNames(spans))
	return nil
}

func entSpanNames(spans []*tracepb.Span) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.GetName())
	}
	return names
}

func spanAttributeValue(span *tracepb.Span, name string) string {
	for _, attr := range span.GetAttributes() {
		if attr.GetKey() == name {
			return attr.GetValue().GetStringValue()
		}
	}
	return ""
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

type trackingDriver struct {
	observabilityTestDriver
	closes int
}

func (d *trackingDriver) Close() error {
	d.closes++
	return nil
}

type recordingDriverPlugin struct {
	order *[]string
}

func (p recordingDriverPlugin) WrapEntDriver(driver dialect.Driver) (dialect.Driver, error) {
	*p.order = append(*p.order, "driver")
	return driver, nil
}

type recordingClientPlugin struct {
	order *[]string
}

func (p recordingClientPlugin) InstallEntClientPlugin(*ent.Client) error {
	*p.order = append(*p.order, "client")
	return nil
}

type replaceDriverPlugin struct {
	driver dialect.Driver
}

func (p replaceDriverPlugin) WrapEntDriver(dialect.Driver) (dialect.Driver, error) {
	return p.driver, nil
}

type failingClientPlugin struct {
	err error
}

func (p failingClientPlugin) InstallEntClientPlugin(*ent.Client) error {
	return p.err
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
