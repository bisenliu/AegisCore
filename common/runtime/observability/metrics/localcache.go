package metrics

import (
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/aegiscore/common/runtime/localcache"
)

const (
	localcacheRequestsMetricName     = "aegiscore_localcache_requests_total"
	localcacheLoadsMetricName        = "aegiscore_localcache_loads_total"
	localcacheSingleflightMetricName = "aegiscore_localcache_singleflight_total"
	localcacheWritesMetricName       = "aegiscore_localcache_writes_total"
	localcacheEvictionsMetricName    = "aegiscore_localcache_evictions_total"
	localcacheCapacityMetricName     = "aegiscore_localcache_capacity"

	localcacheRequestsMetricHelp     = "Total number of localcache business requests by fixed cache and result."
	localcacheLoadsMetricHelp        = "Total number of localcache loader executions by fixed cache and result."
	localcacheSingleflightMetricHelp = "Total number of localcache singleflight events by fixed cache."
	localcacheWritesMetricHelp       = "Total number of localcache write-side events by fixed cache."
	localcacheEvictionsMetricHelp    = "Total number of localcache evictions by fixed cache."
	localcacheCapacityMetricHelp     = "Configured localcache max cost by fixed cache."

	localcacheResultHit        = "hit"
	localcacheResultMiss       = "miss"
	localcacheResultSuccess    = "success"
	localcacheResultError      = "error"
	localcacheEventShared      = "shared"
	localcacheEventDoubleCheck = "double_check_hit"
	localcacheEventSetDropped  = "set_dropped"
	localcacheEventRejected    = "rejected"
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

	requests     *prometheus.Desc
	loads        *prometheus.Desc
	singleflight *prometheus.Desc
	writes       *prometheus.Desc
	evictions    *prometheus.Desc
	capacity     *prometheus.Desc
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
		cache:        cache,
		source:       opts.Source,
		requests:     prometheus.NewDesc(localcacheRequestsMetricName, localcacheRequestsMetricHelp, []string{LabelResult}, prometheus.Labels{LabelCache: cache}),
		loads:        prometheus.NewDesc(localcacheLoadsMetricName, localcacheLoadsMetricHelp, []string{LabelResult}, prometheus.Labels{LabelCache: cache}),
		singleflight: prometheus.NewDesc(localcacheSingleflightMetricName, localcacheSingleflightMetricHelp, []string{LabelEvent}, prometheus.Labels{LabelCache: cache}),
		writes:       prometheus.NewDesc(localcacheWritesMetricName, localcacheWritesMetricHelp, []string{LabelEvent}, prometheus.Labels{LabelCache: cache}),
		evictions:    prometheus.NewDesc(localcacheEvictionsMetricName, localcacheEvictionsMetricHelp, nil, prometheus.Labels{LabelCache: cache}),
		capacity:     prometheus.NewDesc(localcacheCapacityMetricName, localcacheCapacityMetricHelp, nil, prometheus.Labels{LabelCache: cache}),
	}, nil
}

// Describe 实现 prometheus.Collector。
func (c *LocalcacheCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.requests
	ch <- c.loads
	ch <- c.singleflight
	ch <- c.writes
	ch <- c.evictions
	ch <- c.capacity
}

// Collect 实现 prometheus.Collector。
func (c *LocalcacheCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.source.Stats()
	successLoads := stats.Load - stats.LoadError
	if stats.LoadError > stats.Load {
		successLoads = 0
	}
	ch <- prometheus.MustNewConstMetric(c.requests, prometheus.CounterValue, float64(stats.Hit), localcacheResultHit)
	ch <- prometheus.MustNewConstMetric(c.requests, prometheus.CounterValue, float64(stats.Miss), localcacheResultMiss)
	ch <- prometheus.MustNewConstMetric(c.loads, prometheus.CounterValue, float64(successLoads), localcacheResultSuccess)
	ch <- prometheus.MustNewConstMetric(c.loads, prometheus.CounterValue, float64(stats.LoadError), localcacheResultError)
	ch <- prometheus.MustNewConstMetric(c.singleflight, prometheus.CounterValue, float64(stats.Shared), localcacheEventShared)
	ch <- prometheus.MustNewConstMetric(c.singleflight, prometheus.CounterValue, float64(stats.DoubleCheckHit), localcacheEventDoubleCheck)
	ch <- prometheus.MustNewConstMetric(c.writes, prometheus.CounterValue, float64(stats.SetDropped), localcacheEventSetDropped)
	ch <- prometheus.MustNewConstMetric(c.writes, prometheus.CounterValue, float64(stats.Rejected), localcacheEventRejected)
	ch <- prometheus.MustNewConstMetric(c.evictions, prometheus.CounterValue, float64(stats.Evicted))
	ch <- prometheus.MustNewConstMetric(c.capacity, prometheus.GaugeValue, float64(stats.Capacity))
}
