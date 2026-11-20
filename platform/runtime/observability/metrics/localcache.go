package metrics

import (
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/aegiscore/platform/runtime/localcache"
)

const (
	// localcacheRequestsMetricName 是本地缓存请求总数指标名。
	localcacheRequestsMetricName = "aegiscore_localcache_requests_total"
	// localcacheLoadsMetricName 是本地缓存回源加载总数指标名。
	localcacheLoadsMetricName = "aegiscore_localcache_loads_total"
	// localcacheCapacityEvictionsMetricName 是本地缓存因容量达到上限被淘汰条目总数指标名。
	localcacheCapacityEvictionsMetricName = "aegiscore_localcache_capacity_evictions_total"
	// localcacheCapacityMetricName 是本地缓存配置的最大容量指标名。
	localcacheCapacityMetricName = "aegiscore_localcache_capacity"

	// localcacheRequestsMetricHelp 是本地缓存请求总数指标说明。
	localcacheRequestsMetricHelp = "Total number of localcache business requests by fixed cache and result."
	// localcacheLoadsMetricHelp 是本地缓存回源总数指标说明。
	localcacheLoadsMetricHelp = "Total number of localcache loader executions by fixed cache and result."
	// localcacheCapacityEvictionsMetricHelp 是本地缓存淘汰条目指标说明。
	localcacheCapacityEvictionsMetricHelp = "Total number of localcache capacity evictions by fixed cache."
	// localcacheCapacityMetricHelp 是本地缓存容量指标说明。
	localcacheCapacityMetricHelp = "Configured localcache maximum item count by fixed cache."

	// localcacheResultHit 表示本地缓存命中结果标签。
	localcacheResultHit = "hit"
	// localcacheResultMiss 表示本地缓存未命中结果标签。
	localcacheResultMiss = "miss"
	// localcacheResultSuccess 表示本地缓存回源加载成功结果标签。
	localcacheResultSuccess = "success"
	// localcacheResultError 表示本地缓存回源加载失败结果标签。
	localcacheResultError = "error"
)

// LocalcacheCollectorOptions 配置本地缓存指标 collector。
type LocalcacheCollectorOptions struct {
	Cache  string
	Source localcache.StatsSource
}

// LocalcacheCollector 从 localcache.Stats 快照导出指标。
type LocalcacheCollector struct {
	cache  string
	source localcache.StatsSource

	requests  *prometheus.Desc
	loads     *prometheus.Desc
	evictions *prometheus.Desc
	capacity  *prometheus.Desc
}

// NewLocalcacheCollector 构造本地缓存指标 collector。
func NewLocalcacheCollector(opts LocalcacheCollectorOptions) (*LocalcacheCollector, error) {
	if opts.Source == nil {
		return nil, errors.New("localcache metrics source is required")
	}
	cache := strings.TrimSpace(opts.Cache)
	if cache == "" {
		cache = strings.TrimSpace(opts.Source.Name())
	}
	if cache == "" {
		return nil, errors.New("localcache metrics cache is required")
	}

	return &LocalcacheCollector{
		cache:     cache,
		source:    opts.Source,
		requests:  prometheus.NewDesc(localcacheRequestsMetricName, localcacheRequestsMetricHelp, []string{LabelResult}, prometheus.Labels{LabelCache: cache}),
		loads:     prometheus.NewDesc(localcacheLoadsMetricName, localcacheLoadsMetricHelp, []string{LabelResult}, prometheus.Labels{LabelCache: cache}),
		evictions: prometheus.NewDesc(localcacheCapacityEvictionsMetricName, localcacheCapacityEvictionsMetricHelp, nil, prometheus.Labels{LabelCache: cache}),
		capacity:  prometheus.NewDesc(localcacheCapacityMetricName, localcacheCapacityMetricHelp, nil, prometheus.Labels{LabelCache: cache}),
	}, nil
}

// Describe 实现 prometheus.Collector。
func (c *LocalcacheCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.requests
	ch <- c.loads
	ch <- c.evictions
	ch <- c.capacity
}

// Collect 实现 prometheus.Collector。
func (c *LocalcacheCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.source.Stats()
	ch <- prometheus.MustNewConstMetric(c.requests, prometheus.CounterValue, float64(stats.Hit), localcacheResultHit)
	ch <- prometheus.MustNewConstMetric(c.requests, prometheus.CounterValue, float64(stats.Miss), localcacheResultMiss)
	ch <- prometheus.MustNewConstMetric(c.loads, prometheus.CounterValue, float64(stats.LoadSuccess), localcacheResultSuccess)
	ch <- prometheus.MustNewConstMetric(c.loads, prometheus.CounterValue, float64(stats.LoadError), localcacheResultError)
	ch <- prometheus.MustNewConstMetric(c.evictions, prometheus.CounterValue, float64(stats.CapacityEvictions))
	ch <- prometheus.MustNewConstMetric(c.capacity, prometheus.GaugeValue, float64(stats.Capacity))
}
