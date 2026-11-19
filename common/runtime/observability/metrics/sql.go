package metrics

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// sqlOpenConnectionsMetricName 是当前已建立的数据库打开连接数 Gauge 指标名。
	sqlOpenConnectionsMetricName = "aegiscore_postgres_pool_open_connections"
	// sqlInUseConnectionsMetricName 是当前正在使用中的数据库连接数 Gauge 指标名。
	sqlInUseConnectionsMetricName = "aegiscore_postgres_pool_in_use_connections"
	// sqlIdleConnectionsMetricName 是当前处于空闲状态的数据库连接数 Gauge 指标名。
	sqlIdleConnectionsMetricName = "aegiscore_postgres_pool_idle_connections"
	// sqlWaitCountMetricName 是等待空闲连接累计次数 Counter 指标名。
	sqlWaitCountMetricName = "aegiscore_postgres_pool_wait_count_total"
	// sqlWaitDurationMetricName 是等待空闲连接累计耗时 Counter 指标名（单位：秒）。
	sqlWaitDurationMetricName = "aegiscore_postgres_pool_wait_duration_seconds_total"
	// sqlMaxOpenConnectionsMetricName 是配置的连接池最大连接数 Gauge 指标名。
	sqlMaxOpenConnectionsMetricName = "aegiscore_postgres_pool_max_open_connections"
	// sqlOpenConnectionsMetricHelp 是打开连接数指标说明。
	sqlOpenConnectionsMetricHelp = "Current number of established PostgreSQL pool connections."
	// sqlInUseConnectionsMetricHelp 是使用中连接数指标说明。
	sqlInUseConnectionsMetricHelp = "Current number of in-use PostgreSQL pool connections."
	// sqlIdleConnectionsMetricHelp 是空闲连接数指标说明。
	sqlIdleConnectionsMetricHelp = "Current number of idle PostgreSQL pool connections."
	// sqlWaitCountMetricHelp 是等待连接次数指标说明。
	sqlWaitCountMetricHelp = "Total number of PostgreSQL pool waits for a free connection."
	// sqlWaitDurationMetricHelp 是等待连接耗时指标说明。
	sqlWaitDurationMetricHelp = "Total time spent waiting for PostgreSQL pool connections in seconds."
	// sqlMaxOpenConnectionsMetricHelp 是最大连接数配置指标说明。
	sqlMaxOpenConnectionsMetricHelp = "Configured maximum number of open PostgreSQL pool connections."
)

// SQLDBCollectorOptions 配置 PostgreSQL 连接池指标 collector。
type SQLDBCollectorOptions struct {
	Resource string
	DB       *sql.DB
}

// SQLDBCollector 从 sql.DB.Stats 快照导出连接池指标。
type SQLDBCollector struct {
	resource string
	db       *sql.DB

	openConnections    *prometheus.Desc
	inUseConnections   *prometheus.Desc
	idleConnections    *prometheus.Desc
	waitCount          *prometheus.Desc
	waitDuration       *prometheus.Desc
	maxOpenConnections *prometheus.Desc
}

// NewSQLDBCollector 构造 PostgreSQL 连接池指标 collector。
func NewSQLDBCollector(opts SQLDBCollectorOptions) (*SQLDBCollector, error) {
	resource := strings.TrimSpace(opts.Resource)
	if resource == "" {
		return nil, errors.New("sql metrics resource is required")
	}
	if opts.DB == nil {
		return nil, errors.New("sql metrics db is required")
	}

	labels := []string{LabelResource}
	return &SQLDBCollector{
		resource:           resource,
		db:                 opts.DB,
		openConnections:    prometheus.NewDesc(sqlOpenConnectionsMetricName, sqlOpenConnectionsMetricHelp, labels, nil),
		inUseConnections:   prometheus.NewDesc(sqlInUseConnectionsMetricName, sqlInUseConnectionsMetricHelp, labels, nil),
		idleConnections:    prometheus.NewDesc(sqlIdleConnectionsMetricName, sqlIdleConnectionsMetricHelp, labels, nil),
		waitCount:          prometheus.NewDesc(sqlWaitCountMetricName, sqlWaitCountMetricHelp, labels, nil),
		waitDuration:       prometheus.NewDesc(sqlWaitDurationMetricName, sqlWaitDurationMetricHelp, labels, nil),
		maxOpenConnections: prometheus.NewDesc(sqlMaxOpenConnectionsMetricName, sqlMaxOpenConnectionsMetricHelp, labels, nil),
	}, nil
}

// Describe 实现 prometheus.Collector。
func (c *SQLDBCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.openConnections
	ch <- c.inUseConnections
	ch <- c.idleConnections
	ch <- c.waitCount
	ch <- c.waitDuration
	ch <- c.maxOpenConnections
}

// Collect 实现 prometheus.Collector。
func (c *SQLDBCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.db.Stats()
	ch <- prometheus.MustNewConstMetric(c.openConnections, prometheus.GaugeValue, float64(stats.OpenConnections), c.resource)
	ch <- prometheus.MustNewConstMetric(c.inUseConnections, prometheus.GaugeValue, float64(stats.InUse), c.resource)
	ch <- prometheus.MustNewConstMetric(c.idleConnections, prometheus.GaugeValue, float64(stats.Idle), c.resource)
	ch <- prometheus.MustNewConstMetric(c.waitCount, prometheus.CounterValue, float64(stats.WaitCount), c.resource)
	ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, stats.WaitDuration.Seconds(), c.resource)
	ch <- prometheus.MustNewConstMetric(c.maxOpenConnections, prometheus.GaugeValue, float64(stats.MaxOpenConnections), c.resource)
}
