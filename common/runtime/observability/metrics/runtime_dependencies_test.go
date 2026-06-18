package metrics

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	io_prometheus_client "github.com/prometheus/client_model/go"

	"github.com/aegiscore/common/runtime/scheduler"
	"github.com/aegiscore/common/runtime/workerpool"
)

func TestSQLDBCollectorExportsPoolStats(t *testing.T) {
	provider := newTestProvider(t, true, false)
	db := sql.OpenDB(noopConnector{})
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(7)

	collector, err := NewSQLDBCollector(SQLDBCollectorOptions{Resource: "user_db", DB: db})
	if err != nil {
		t.Fatalf("NewSQLDBCollector: %v", err)
	}
	if err := provider.Register(collector); err != nil {
		t.Fatalf("Register: %v", err)
	}

	metric := firstMetric(t, gatherFamily(t, provider, sqlMaxOpenConnectionsMetricName))
	assertMetricLabel(t, metric, LabelResource, "user_db")
	if got := metric.GetGauge().GetValue(); got != 7 {
		t.Fatalf("max open gauge = %v, want 7", got)
	}
	assertHasFamily(t, provider, sqlOpenConnectionsMetricName)
	assertHasFamily(t, provider, sqlWaitCountMetricName)
	assertHasFamily(t, provider, sqlWaitDurationMetricName)
}

func TestSQLDBCollectorRejectsInvalidOptions(t *testing.T) {
	if _, err := NewSQLDBCollector(SQLDBCollectorOptions{}); err == nil || !strings.Contains(err.Error(), "resource") {
		t.Fatalf("empty resource error = %v, want resource error", err)
	}
	if _, err := NewSQLDBCollector(SQLDBCollectorOptions{Resource: "user_db"}); err == nil || !strings.Contains(err.Error(), "db") {
		t.Fatalf("nil db error = %v, want db error", err)
	}
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
		if err != nil {
			t.Fatalf("NewRedisPingCollector: %v", err)
		}
		if err := provider.Register(collector); err != nil {
			t.Fatalf("Register: %v", err)
		}

		families := gatherFamilies(t, provider)
		up := firstMetric(t, familyFrom(t, families, redisUpMetricName))
		assertMetricLabel(t, up, LabelResource, "cache_redis")
		if got := up.GetGauge().GetValue(); got != 1 {
			t.Fatalf("redis up = %v, want 1", got)
		}
		failures := firstMetric(t, familyFrom(t, families, redisPingFailuresMetricName))
		if got := failures.GetCounter().GetValue(); got != 0 {
			t.Fatalf("redis failures = %v, want 0", got)
		}
		gatherFamilies(t, provider)
		if got := pinger.calls.Load(); got != 1 {
			t.Fatalf("redis ping calls = %d, want cached single probe", got)
		}
	})

	t.Run("failure", func(t *testing.T) {
		provider := newTestProvider(t, true, false)
		collector, err := NewRedisPingCollector(RedisPingCollectorOptions{
			Resource: "cache_redis",
			Pinger:   fakeRedisPinger{err: errors.New("ping failed")},
			Timeout:  time.Second,
		})
		if err != nil {
			t.Fatalf("NewRedisPingCollector: %v", err)
		}
		if err := provider.Register(collector); err != nil {
			t.Fatalf("Register: %v", err)
		}

		families := gatherFamilies(t, provider)
		up := firstMetric(t, familyFrom(t, families, redisUpMetricName))
		if got := up.GetGauge().GetValue(); got != 0 {
			t.Fatalf("redis up = %v, want 0", got)
		}
		failures := firstMetric(t, familyFrom(t, families, redisPingFailuresMetricName))
		if got := failures.GetCounter().GetValue(); got != 1 {
			t.Fatalf("redis failures = %v, want 1", got)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		provider := newTestProvider(t, true, false)
		collector, err := NewRedisPingCollector(RedisPingCollectorOptions{
			Resource: "cache_redis",
			Pinger:   blockingRedisPinger{},
			Timeout:  10 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("NewRedisPingCollector: %v", err)
		}
		if err := provider.Register(collector); err != nil {
			t.Fatalf("Register: %v", err)
		}

		up := firstMetric(t, gatherFamily(t, provider, redisUpMetricName))
		if got := up.GetGauge().GetValue(); got != 0 {
			t.Fatalf("redis up = %v, want 0", got)
		}
	})
}

func TestWorkerpoolCollectorExportsStatsSnapshot(t *testing.T) {
	provider := newTestProvider(t, true, false)
	source := fakeWorkerpoolStatsSource{stats: workerpool.Stats{
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
	if err != nil {
		t.Fatalf("NewWorkerpoolCollector: %v", err)
	}
	if err := provider.Register(collector); err != nil {
		t.Fatalf("Register: %v", err)
	}

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
	if _, ok := recorder.(scheduler.NopMetrics); !ok {
		t.Fatalf("recorder type = %T, want scheduler.NopMetrics", recorder)
	}
}

func TestComponentStatusCollectorExportsRunningAndLastError(t *testing.T) {
	provider := newTestProvider(t, true, false)
	collector, err := NewComponentStatusCollector(ComponentStatusCollectorOptions{
		Resource: "rbac_policy_watcher",
		Source:   fakeComponentStatus{running: true, err: errors.New("subscribe failed")},
	})
	if err != nil {
		t.Fatalf("NewComponentStatusCollector: %v", err)
	}
	if err := provider.Register(collector); err != nil {
		t.Fatalf("Register: %v", err)
	}

	running := firstMetric(t, gatherFamily(t, provider, statusRunningMetricName))
	assertMetricLabel(t, running, LabelResource, "rbac_policy_watcher")
	if got := running.GetGauge().GetValue(); got != 1 {
		t.Fatalf("running = %v, want 1", got)
	}
	lastErr := firstMetric(t, gatherFamily(t, provider, statusLastErrorMetricName))
	if got := lastErr.GetGauge().GetValue(); got != 1 {
		t.Fatalf("last error = %v, want 1", got)
	}
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

type fakeRedisPinger struct {
	err error
}

func (p fakeRedisPinger) Ping(context.Context) error {
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

type fakeWorkerpoolStatsSource struct {
	stats workerpool.Stats
}

func (s fakeWorkerpoolStatsSource) Stats() workerpool.Stats {
	return s.stats
}

type fakeComponentStatus struct {
	running bool
	err     error
}

func (s fakeComponentStatus) Running() bool {
	return s.running
}

func (s fakeComponentStatus) LastError() error {
	return s.err
}

func assertGaugeValue(t *testing.T, family *io_prometheus_client.MetricFamily, want float64) {
	t.Helper()
	if got := firstMetric(t, family).GetGauge().GetValue(); got != want {
		t.Fatalf("%s gauge = %v, want %v", family.GetName(), got, want)
	}
}

func assertMetricWithLabelsValue(t *testing.T, family *io_prometheus_client.MetricFamily, labels map[string]string, want float64) {
	t.Helper()
	metric := findMetricWithLabels(t, family, labels)
	if got := metricValue(metric); got != want {
		t.Fatalf("%s labels %#v value = %v, want %v", family.GetName(), labels, got, want)
	}
}

func assertHistogramSampleCount(t *testing.T, family *io_prometheus_client.MetricFamily, labels map[string]string, want uint64) {
	t.Helper()
	metric := findMetricWithLabels(t, family, labels)
	if got := metric.GetHistogram().GetSampleCount(); got != want {
		t.Fatalf("%s labels %#v sample count = %d, want %d", family.GetName(), labels, got, want)
	}
}

func findMetricWithLabels(t *testing.T, family *io_prometheus_client.MetricFamily, labels map[string]string) *io_prometheus_client.Metric {
	t.Helper()
	for _, metric := range family.GetMetric() {
		if metricHasLabels(metric, labels) {
			return metric
		}
	}
	t.Fatalf("metric family %q missing labels %#v", family.GetName(), labels)
	return nil
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
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	return families
}

func familyFrom(t *testing.T, families []*io_prometheus_client.MetricFamily, name string) *io_prometheus_client.MetricFamily {
	t.Helper()
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	t.Fatalf("metric family %q not found", name)
	return nil
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
