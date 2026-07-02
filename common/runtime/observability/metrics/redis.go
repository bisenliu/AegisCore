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
	redisUpMetricName              = "aegiscore_redis_up"
	redisPingDurationMetricName    = "aegiscore_redis_ping_duration_seconds"
	redisPingFailuresMetricName    = "aegiscore_redis_ping_failures_total"
	redisUpMetricHelp              = "Whether the Redis resource responded to the latest cached ping probe."
	redisPingDurationMetricHelp    = "Duration of the latest Redis ping probe in seconds."
	redisPingFailuresMetricHelp    = "Total number of failed Redis ping probes."
	defaultRedisPingMetricsTimeout = time.Second
	defaultRedisPingMinInterval    = 15 * time.Second
)

// RedisPinger 定义 Redis ping 指标需要的最小依赖。
type RedisPinger interface {
	Ping(ctx context.Context) error
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
	client *redis.Client
}

// NewRedisClientPinger 将 go-redis client 适配为 RedisPinger。
func NewRedisClientPinger(client *redis.Client) RedisPinger {
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

func (c *RedisPingCollector) snapshot(parent context.Context) redisPingSnapshot {
	if parent == nil {
		parent = context.Background()
	}
	now := time.Now()
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

func (p redisClientPinger) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
