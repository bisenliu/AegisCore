package metrics

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/scheduler"
	"github.com/aegiscore/common/runtime/workerpool"
)

func TestSQLDBCollectorExportsPoolStats(t *testing.T) {
	provider := newTestProvider(t, true, false)
	db := sql.OpenDB(noopConnector{})
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(7)

	collector, err := NewSQLDBCollector(SQLDBCollectorOptions{Resource: "user_db", DB: db})
	require.NoError(t, err, "NewSQLDBCollector")
	require.NoError(t, provider.Register(collector), "Register")

	metric := firstMetric(t, gatherFamily(t, provider, sqlMaxOpenConnectionsMetricName))
	assertMetricLabel(t, metric, LabelResource, "user_db")
	require.Equal(t, float64(7), metric.GetGauge().GetValue(), "max open gauge")
	assertHasFamily(t, provider, sqlOpenConnectionsMetricName)
	assertHasFamily(t, provider, sqlWaitCountMetricName)
	assertHasFamily(t, provider, sqlWaitDurationMetricName)
}

func TestSQLDBCollectorRejectsInvalidOptions(t *testing.T) {
	_, err := NewSQLDBCollector(SQLDBCollectorOptions{})
	require.ErrorContains(t, err, "resource")
	_, err = NewSQLDBCollector(SQLDBCollectorOptions{Resource: "user_db"})
	require.ErrorContains(t, err, "db")
}

func TestRedisPingCollectorExportsSuccessFailureAndTimeout(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		provider := newTestProvider(t, true, false)
		pinger := &countingRedisPinger{}
		collector, err := NewRedisPingCollector(RedisPingCollectorOptions{
			Resource:    "cache_redis",
			Pinger:      pinger,
			Timeout:     time.Second,
			MinInterval: time.Minute,
		})
		require.NoError(t, err, "NewRedisPingCollector")
		require.NoError(t, provider.Register(collector), "Register")

		families := gatherFamilies(t, provider)
		up := firstMetric(t, familyFrom(t, families, redisUpMetricName))
		assertMetricLabel(t, up, LabelResource, "cache_redis")
		require.Equal(t, float64(1), up.GetGauge().GetValue(), "redis up")
		failures := firstMetric(t, familyFrom(t, families, redisPingFailuresMetricName))
		require.Zero(t, failures.GetCounter().GetValue(), "redis failures")
		gatherFamilies(t, provider)
		require.EqualValues(t, 1, pinger.calls.Load(), "redis ping calls want cached single probe")
	})

	t.Run("failure", func(t *testing.T) {
		provider := newTestProvider(t, true, false)
		collector, err := NewRedisPingCollector(RedisPingCollectorOptions{
			Resource: "cache_redis",
			Pinger:   staticRedisPinger{err: errors.New("ping failed")},
			Timeout:  time.Second,
		})
		require.NoError(t, err, "NewRedisPingCollector")
		require.NoError(t, provider.Register(collector), "Register")

		families := gatherFamilies(t, provider)
		up := firstMetric(t, familyFrom(t, families, redisUpMetricName))
		require.Zero(t, up.GetGauge().GetValue(), "redis up")
		failures := firstMetric(t, familyFrom(t, families, redisPingFailuresMetricName))
		require.Equal(t, float64(1), failures.GetCounter().GetValue(), "redis failures")
	})

	t.Run("timeout", func(t *testing.T) {
		provider := newTestProvider(t, true, false)
		collector, err := NewRedisPingCollector(RedisPingCollectorOptions{
			Resource: "cache_redis",
			Pinger:   blockingRedisPinger{},
			Timeout:  10 * time.Millisecond,
		})
		require.NoError(t, err, "NewRedisPingCollector")
		require.NoError(t, provider.Register(collector), "Register")

		up := firstMetric(t, gatherFamily(t, provider, redisUpMetricName))
		require.Zero(t, up.GetGauge().GetValue(), "redis up")
	})
}

func TestRedisPingCollectorUsesGatherContextCancellation(t *testing.T) {
	provider := newTestProvider(t, true, false)
	pinger := newObservingRedisPinger()
	collector, err := NewRedisPingCollector(RedisPingCollectorOptions{
		Resource:    "cache_redis",
		Pinger:      pinger,
		Timeout:     time.Second,
		MinInterval: time.Nanosecond,
	})
	require.NoError(t, err, "NewRedisPingCollector")
	require.NoError(t, provider.Register(collector), "Register")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := provider.GatherContext(ctx)
		done <- err
	}()

	select {
	case <-pinger.started:
	case <-time.After(time.Second):
		t.Fatal("redis ping did not start")
	}
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err, "GatherContext")
	case <-time.After(time.Second):
		t.Fatal("GatherContext did not finish after request context cancellation")
	}
	require.ErrorIs(t, pinger.ctxErr(), context.Canceled, "redis ping context error")
	up := firstMetric(t, gatherFamily(t, provider, redisUpMetricName))
	require.Zero(t, up.GetGauge().GetValue(), "redis up after canceled ping")
}

func TestRedisPingCollectorDirectCollectUsesBackgroundFallback(t *testing.T) {
	pinger := newObservingRedisPinger()
	collector, err := NewRedisPingCollector(RedisPingCollectorOptions{
		Resource:    "cache_redis",
		Pinger:      pinger,
		Timeout:     10 * time.Millisecond,
		MinInterval: time.Nanosecond,
	})
	require.NoError(t, err, "NewRedisPingCollector")

	ch := make(chan prometheus.Metric, 3)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	select {
	case <-pinger.started:
	case <-time.After(time.Second):
		t.Fatal("redis ping did not start")
	}

	var collected []prometheus.Metric
	select {
	case metric := <-ch:
		collected = append(collected, metric)
	case <-time.After(time.Second):
		t.Fatal("Collect did not finish after collector timeout")
	}
	for metric := range ch {
		collected = append(collected, metric)
	}
	require.Len(t, collected, 3, "direct Collect metrics")
	dtoMetric := &io_prometheus_client.Metric{}
	require.NoError(t, collected[0].Write(dtoMetric), "Write metric")
	assertMetricLabel(t, dtoMetric, LabelResource, "cache_redis")
	require.ErrorIs(t, pinger.ctxErr(), context.DeadlineExceeded, "direct Collect context error")
}

func TestWorkerpoolCollectorExportsStatsSnapshot(t *testing.T) {
	provider := newTestProvider(t, true, false)
	source := staticWorkerpoolStatsSource{stats: workerpool.Stats{
		Name:      "auth.redis.session_purge",
		Workers:   4,
		Submitted: 10,
		Rejected:  2,
		Started:   8,
		Completed: 6,
		Failed:    1,
		Panicked:  1,
		Queued:    2,
		Running:   3,
		Free:      1,
		Waiting:   5,
		Closed:    true,
	}}
	collector, err := NewWorkerpoolCollector(WorkerpoolCollectorOptions{Pool: "auth_session_purge_pool", Source: source})
	require.NoError(t, err, "NewWorkerpoolCollector")
	require.NoError(t, provider.Register(collector), "Register")

	tasks := gatherFamily(t, provider, workerpoolTasksMetricName)
	assertMetricWithLabelsValue(t, tasks, map[string]string{LabelPool: "auth_session_purge_pool", LabelEvent: workerpoolEventSubmitted}, 10)
	assertMetricWithLabelsValue(t, tasks, map[string]string{LabelPool: "auth_session_purge_pool", LabelEvent: workerpoolEventRejected}, 2)
	assertMetricWithLabelsValue(t, tasks, map[string]string{LabelPool: "auth_session_purge_pool", LabelEvent: workerpoolEventPanicked}, 1)
	assertGaugeValue(t, gatherFamily(t, provider, workerpoolQueuedMetricName), 2)
	assertGaugeValue(t, gatherFamily(t, provider, workerpoolRunningMetricName), 3)
	assertGaugeValue(t, gatherFamily(t, provider, workerpoolWaitingMetricName), 5)
}

func TestSchedulerMetricsAdapterRecordsEvents(t *testing.T) {
	provider := newTestProvider(t, true, false)
	recorder := NewSchedulerMetrics(provider, SchedulerMetricsOptions{DurationBuckets: []float64{0.01, 0.1, 1}})

	recorder.JobTriggered("rbac_policy_version_check")
	recorder.JobStarted("rbac_policy_version_check")
	recorder.JobCompleted("rbac_policy_version_check", 50*time.Millisecond)
	recorder.JobFailed("rbac_policy_version_check", 75*time.Millisecond)
	recorder.JobSkipped("rbac_policy_version_check", "lock_busy")
	recorder.JobLockRenewFailed("rbac_policy_version_check")

	jobs := gatherFamily(t, provider, schedulerJobsMetricName)
	assertMetricWithLabelsValue(t, jobs, map[string]string{
		LabelSchedulerJob: "rbac_policy_version_check",
		LabelEvent:        schedulerEventCompleted,
		LabelStatus:       schedulerStatusSuccess,
		LabelReason:       schedulerReasonNone,
	}, 1)
	assertMetricWithLabelsValue(t, jobs, map[string]string{
		LabelSchedulerJob: "rbac_policy_version_check",
		LabelEvent:        schedulerEventSkipped,
		LabelStatus:       schedulerStatusSkipped,
		LabelReason:       "lock_busy",
	}, 1)
	assertMetricWithLabelsValue(t, jobs, map[string]string{
		LabelSchedulerJob: "rbac_policy_version_check",
		LabelEvent:        schedulerEventLockRenewFailed,
		LabelStatus:       schedulerStatusFailure,
		LabelReason:       schedulerReasonNone,
	}, 1)

	duration := gatherFamily(t, provider, schedulerJobDurationMetricName)
	assertHistogramSampleCount(t, duration, map[string]string{LabelSchedulerJob: "rbac_policy_version_check", LabelStatus: schedulerStatusSuccess}, 1)
	assertHistogramSampleCount(t, duration, map[string]string{LabelSchedulerJob: "rbac_policy_version_check", LabelStatus: schedulerStatusFailure}, 1)
}

func TestSchedulerMetricsAdapterDisabledProviderReturnsNop(t *testing.T) {
	recorder := NewSchedulerMetrics(newTestProvider(t, false, false), SchedulerMetricsOptions{})
	require.IsType(t, scheduler.NopMetrics{}, recorder)
}

func TestSchedulerMetricsAdapterUsesDefaultDurationBuckets(t *testing.T) {
	provider := newTestProvider(t, true, false)
	recorder := NewSchedulerMetrics(provider, SchedulerMetricsOptions{})
	recorder.JobCompleted("job", 50*time.Millisecond)

	metric := firstMetric(t, gatherFamily(t, provider, schedulerJobDurationMetricName))
	buckets := metric.GetHistogram().GetBucket()
	require.Len(t, buckets, 11)
	wants := []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}
	for index, want := range wants {
		require.Equal(t, want, buckets[index].GetUpperBound())
	}
}

func TestComponentStatusCollectorExportsRunningAndLastError(t *testing.T) {
	provider := newTestProvider(t, true, false)
	collector, err := NewComponentStatusCollector(ComponentStatusCollectorOptions{
		Resource: "rbac_policy_watcher",
		Source:   staticComponentStatusSource{running: true, err: errors.New("subscribe failed")},
	})
	require.NoError(t, err, "NewComponentStatusCollector")
	require.NoError(t, provider.Register(collector), "Register")

	running := firstMetric(t, gatherFamily(t, provider, statusRunningMetricName))
	assertMetricLabel(t, running, LabelResource, "rbac_policy_watcher")
	require.Equal(t, float64(1), running.GetGauge().GetValue(), "running")
	lastErr := firstMetric(t, gatherFamily(t, provider, statusLastErrorMetricName))
	require.Equal(t, float64(1), lastErr.GetGauge().GetValue(), "last error")
}

func TestCasbinPolicyReloadMetricsRecordsStatus(t *testing.T) {
	provider := newTestProvider(t, true, false)
	recorder := NewCasbinPolicyReloadMetrics(provider)
	recorder.ReloadSucceeded()
	recorder.SetLastStatus(true)
	recorder.ReloadFailed()
	recorder.SetLastStatus(false)

	reloads := gatherFamily(t, provider, casbinReloadsMetricName)
	assertMetricWithLabelsValue(t, reloads, map[string]string{LabelStatus: StatusSuccess}, 1)
	assertMetricWithLabelsValue(t, reloads, map[string]string{LabelStatus: StatusFailure}, 1)
	lastStatus := gatherFamily(t, provider, casbinReloadLastMetricName)
	assertGaugeValue(t, lastStatus, 0)
}

type staticRedisPinger struct {
	err error
}

func (p staticRedisPinger) Ping(context.Context) error {
	return p.err
}

type countingRedisPinger struct {
	calls atomic.Uint64
}

func (p *countingRedisPinger) Ping(context.Context) error {
	p.calls.Add(1)
	return nil
}

type blockingRedisPinger struct{}

func (blockingRedisPinger) Ping(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

type observingRedisPinger struct {
	started chan struct{}
	once    sync.Once
	err     error
	mu      sync.Mutex
}

func newObservingRedisPinger() *observingRedisPinger {
	return &observingRedisPinger{started: make(chan struct{})}
}

func (p *observingRedisPinger) Ping(ctx context.Context) error {
	p.once.Do(func() { close(p.started) })
	<-ctx.Done()
	p.mu.Lock()
	p.err = ctx.Err()
	p.mu.Unlock()
	return ctx.Err()
}

func (p *observingRedisPinger) ctxErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

type staticWorkerpoolStatsSource struct {
	stats workerpool.Stats
}

func (s staticWorkerpoolStatsSource) Stats() workerpool.Stats {
	return s.stats
}

type staticComponentStatusSource struct {
	running bool
	err     error
}

func (s staticComponentStatusSource) Running() bool {
	return s.running
}

func (s staticComponentStatusSource) LastError() error {
	return s.err
}

func assertGaugeValue(t *testing.T, family *io_prometheus_client.MetricFamily, want float64) {
	t.Helper()
	require.Equalf(t, want, firstMetric(t, family).GetGauge().GetValue(), "%s gauge", family.GetName())
}

func assertMetricWithLabelsValue(t *testing.T, family *io_prometheus_client.MetricFamily, labels map[string]string, want float64) {
	t.Helper()
	metric := findMetricWithLabels(t, family, labels)
	require.Equalf(t, want, metricValue(metric), "%s labels %#v value", family.GetName(), labels)
}

func assertHistogramSampleCount(t *testing.T, family *io_prometheus_client.MetricFamily, labels map[string]string, want uint64) {
	t.Helper()
	metric := findMetricWithLabels(t, family, labels)
	require.Equalf(t, want, metric.GetHistogram().GetSampleCount(), "%s labels %#v sample count", family.GetName(), labels)
}

func findMetricWithLabels(t *testing.T, family *io_prometheus_client.MetricFamily, labels map[string]string) *io_prometheus_client.Metric {
	t.Helper()
	var found *io_prometheus_client.Metric
	for _, metric := range family.GetMetric() {
		if metricHasLabels(metric, labels) {
			found = metric
			break
		}
	}
	require.NotNilf(t, found, "metric family %q missing labels %#v", family.GetName(), labels)
	return found
}

func metricHasLabels(metric *io_prometheus_client.Metric, labels map[string]string) bool {
	for key, want := range labels {
		found := false
		for _, label := range metric.GetLabel() {
			if label.GetName() == key && label.GetValue() == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func metricValue(metric *io_prometheus_client.Metric) float64 {
	switch {
	case metric.GetCounter() != nil:
		return metric.GetCounter().GetValue()
	case metric.GetGauge() != nil:
		return metric.GetGauge().GetValue()
	default:
		return 0
	}
}

func gatherFamilies(t *testing.T, provider *Provider) []*io_prometheus_client.MetricFamily {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	require.NoError(t, err, "Gather")
	return families
}

func familyFrom(t *testing.T, families []*io_prometheus_client.MetricFamily, name string) *io_prometheus_client.MetricFamily {
	t.Helper()
	var found *io_prometheus_client.MetricFamily
	for _, family := range families {
		if family.GetName() == name {
			found = family
			break
		}
	}
	require.NotNilf(t, found, "metric family %q not found", name)
	return found
}

type noopConnector struct{}

func (noopConnector) Connect(context.Context) (driver.Conn, error) {
	return noopConn{}, nil
}

func (noopConnector) Driver() driver.Driver {
	return noopDriver{}
}

type noopDriver struct{}

func (noopDriver) Open(string) (driver.Conn, error) {
	return noopConn{}, nil
}

type noopConn struct{}

func (noopConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (noopConn) Close() error {
	return nil
}

func (noopConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin is not supported")
}
