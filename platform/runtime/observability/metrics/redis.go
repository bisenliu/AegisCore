package metrics

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

const (
	// redisUpMetricName 是 Redis 存活状态 Gauge 指标名（1 代表可用，0 代表不可用）。
	redisUpMetricName = "aegiscore_redis_up"
	// redisPingDurationMetricName 是最近一次探测 Redis 耗时 Gauge 指标名（单位：秒）。
	redisPingDurationMetricName = "aegiscore_redis_ping_duration_seconds"
	// redisPingFailuresMetricName 是探测 Redis 失败累计次数 Counter 指标名。
	redisPingFailuresMetricName = "aegiscore_redis_ping_failures_total"
	// redisUpMetricHelp 是 Redis 存活状态指标说明。
	redisUpMetricHelp = "Whether the Redis resource responded to the latest cached ping probe."
	// redisPingDurationMetricHelp 是 Redis 探测耗时指标说明。
	redisPingDurationMetricHelp = "Duration of the latest Redis ping probe in seconds."
	// redisPingFailuresMetricHelp 是 Redis 探测失败总数指标说明。
	redisPingFailuresMetricHelp = "Total number of failed Redis ping probes."
	// defaultRedisPingMetricsTimeout 是指标采集探测 Redis 的默认最长等待超时。
	defaultRedisPingMetricsTimeout = time.Second
	// defaultRedisPingMinInterval 是避免 Prometheus 高频刮削导致探测泛滥的最小采样间隔。
	defaultRedisPingMinInterval = 15 * time.Second
)

// RedisPinger 定义 Redis ping 指标需要的最小依赖。
type RedisPinger interface {
	Ping(ctx context.Context) error
}

type redisStatusPinger interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

// RedisPingCollectorOptions 配置 Redis ping 指标 collector。
type RedisPingCollectorOptions struct {
	Resource    string
	Pinger      RedisPinger
	Timeout     time.Duration
	MinInterval time.Duration
}

// RedisPingCollector 在 scrape 时执行 Redis PING 并导出基础可用性指标。
type RedisPingCollector struct {
	resource string
	pinger   RedisPinger
	timeout  time.Duration
	interval time.Duration
	failures atomic.Uint64
	mu       sync.Mutex
	last     redisPingSnapshot

	up           *prometheus.Desc
	pingDuration *prometheus.Desc
	pingFailures *prometheus.Desc
}

type redisPingSnapshot struct {
	up         float64
	duration   time.Duration
	observedAt time.Time
}

type redisClientPinger struct {
	client redisStatusPinger
}

// NewRedisClientPinger 将 go-redis client 适配为 RedisPinger。
func NewRedisClientPinger(client redisStatusPinger) RedisPinger {
	if client == nil {
		return nil
	}
	return redisClientPinger{client: client}
}

// NewRedisPingCollector 构造 Redis ping 指标 collector。
func NewRedisPingCollector(opts RedisPingCollectorOptions) (*RedisPingCollector, error) {
	resource := strings.TrimSpace(opts.Resource)
	if resource == "" {
		return nil, errors.New("redis metrics resource is required")
	}
	if opts.Pinger == nil {
		return nil, errors.New("redis metrics pinger is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultRedisPingMetricsTimeout
	}
	interval := opts.MinInterval
	if interval <= 0 {
		interval = defaultRedisPingMinInterval
	}

	labels := []string{LabelResource}
	return &RedisPingCollector{
		resource:     resource,
		pinger:       opts.Pinger,
		timeout:      timeout,
		interval:     interval,
		up:           prometheus.NewDesc(redisUpMetricName, redisUpMetricHelp, labels, nil),
		pingDuration: prometheus.NewDesc(redisPingDurationMetricName, redisPingDurationMetricHelp, labels, nil),
		pingFailures: prometheus.NewDesc(redisPingFailuresMetricName, redisPingFailuresMetricHelp, labels, nil),
	}, nil
}

// Describe 实现 prometheus.Collector。
func (c *RedisPingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.pingDuration
	ch <- c.pingFailures
}

// Collect 实现 prometheus.Collector。
// Prometheus 标准 Collector 不携带 scrape context；真实 HTTP scrape 取消只在通过
// Provider.GatherContext 或 Provider.HTTPHandler 采集时传递给 CollectContext。
func (c *RedisPingCollector) Collect(ch chan<- prometheus.Metric) {
	c.CollectContext(context.Background(), ch)
}

// CollectContext 使用调用方 context 执行 Redis PING 并导出指标。
func (c *RedisPingCollector) CollectContext(ctx context.Context, ch chan<- prometheus.Metric) {
	snapshot := c.snapshot(ctx)
	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, snapshot.up, c.resource)
	ch <- prometheus.MustNewConstMetric(c.pingDuration, prometheus.GaugeValue, snapshot.duration.Seconds(), c.resource)
	ch <- prometheus.MustNewConstMetric(c.pingFailures, prometheus.CounterValue, float64(c.failures.Load()), c.resource)
}

// snapshot 检查并记录一次与缓存服务器的连接状态，用于在监控看板中展示缓存是否在线。
func (c *RedisPingCollector) snapshot(parent context.Context) redisPingSnapshot {
	if parent == nil {
		parent = context.Background()
	}
	now := time.Now()
	// 持锁覆盖探测，确保并发 scrape 只发起一次 Redis ping，并在 interval 内共享同一快照。
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.last.observedAt.IsZero() && now.Sub(c.last.observedAt) < c.interval {
		return c.last
	}

	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	err := c.pinger.Ping(ctx)
	cancel()

	up := 1.0
	if err != nil {
		up = 0
		c.failures.Add(1)
	}
	c.last = redisPingSnapshot{
		up:         up,
		duration:   time.Since(startedAt),
		observedAt: now,
	}
	return c.last
}

// Ping 将 go-redis StatusCmd 适配为 RedisPinger 的 error-only 契约。
func (p redisClientPinger) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
