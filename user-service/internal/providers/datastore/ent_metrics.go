package datastore

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	platformmetrics "github.com/aegiscore/platform/runtime/observability/metrics"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

const (
	// entQueryLatencyMetricName 是 Ent ORM 查询执行耗时 Histogram 指标名（单位：秒）。
	entQueryLatencyMetricName = "aegiscore_user_service_ent_query_duration_seconds"
	// entQueryErrorMetricName 是 Ent ORM 查询发生错误累计 Counter 指标名。
	entQueryErrorMetricName = "aegiscore_user_service_ent_query_errors_total"
	// entQueryLatencyMetricHelp 是 Ent 查询耗时指标说明。
	entQueryLatencyMetricHelp = "Ent query latency in seconds by fixed entity, query, and result."
	// entQueryErrorMetricHelp 是 Ent 查询错误指标说明。
	entQueryErrorMetricHelp = "Total number of Ent query errors by fixed entity and query."

	// entQueryOperation 是 ORM 查询操作标准标签。
	entQueryOperation = "select"
	// entResultSuccess 是 ORM 查询成功标签。
	entResultSuccess = "success"
	// entResultError 是 ORM 查询失败标签。
	entResultError = "error"
)

// entMetricsPlugin 在 Ent client 上安装 Prometheus query metrics interceptor。
type entMetricsPlugin struct {
	metrics *entQueryMetrics
}

// InstallEntClientPlugin 安装 Ent query metrics；metrics 为空时保持 no-op。
func (p entMetricsPlugin) InstallEntClientPlugin(client *ent.Client) error {
	installEntQueryMetrics(client, p.metrics)
	return nil
}

// entQueryMetrics 持有 Ent query latency histogram 和 error counter。
type entQueryMetrics struct {
	latency *prometheus.HistogramVec
	errors  *prometheus.CounterVec
}

// newEntQueryMetrics 仅在 metrics provider 启用时注册 Ent query collector。
func newEntQueryMetrics(provider *platformmetrics.Provider) (*entQueryMetrics, error) {
	if provider == nil || !provider.Enabled() {
		return nil, nil
	}
	metrics := &entQueryMetrics{
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    entQueryLatencyMetricName,
			Help:    entQueryLatencyMetricHelp,
			Buckets: prometheus.DefBuckets,
		}, []string{"entity", "query", "result"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: entQueryErrorMetricName,
			Help: entQueryErrorMetricHelp,
		}, []string{"entity", "query"}),
	}
	if err := provider.Register(metrics.latency); err != nil {
		return nil, fmt.Errorf("register ent query latency metrics: %w", err)
	}
	if err := provider.Register(metrics.errors); err != nil {
		return nil, fmt.Errorf("register ent query error metrics: %w", err)
	}
	for _, entity := range entQueryMetricEntities() {
		metrics.errors.WithLabelValues(entity, entQueryOperation).Add(0)
	}
	return metrics, nil
}

// installEntQueryMetrics 记录 Ent query latency 和 error，不改变 query 结果或错误传播。
func installEntQueryMetrics(client *ent.Client, metrics *entQueryMetrics) {
	if client == nil || metrics == nil {
		return
	}
	client.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
			entity := entQueryEntity(query)
			startedAt := time.Now()
			value, err := next.Query(ctx, query)
			result := entResultSuccess
			if err != nil {
				result = entResultError
			}
			metrics.latency.WithLabelValues(entity, entQueryOperation, result).Observe(time.Since(startedAt).Seconds())
			if err != nil {
				metrics.errors.WithLabelValues(entity, entQueryOperation).Inc()
			}
			return value, err
		})
	}))
}

// entQueryEntity 将 Ent query 类型映射为固定低基数实体标签。
func entQueryEntity(query ent.Query) string {
	if query == nil {
		return "unknown"
	}
	typeName := reflect.TypeOf(query).String()
	return entEntityFromTypeName(typeName, "Query")
}

// entEntityFromTypeName 将 Ent 生成类型名转换为稳定 snake_case 实体名。
func entEntityFromTypeName(typeName string, suffix string) string {
	if idx := strings.LastIndex(typeName, "."); idx >= 0 {
		typeName = typeName[idx+1:]
	}
	typeName = strings.TrimPrefix(typeName, "*")
	typeName = strings.TrimSuffix(typeName, suffix)
	switch typeName {
	case "Permission":
		return "permission"
	case "Role":
		return "role"
	case "RolePermission":
		return "role_permission"
	case "User":
		return "user"
	case "UserRole":
		return "user_role"
	default:
		return "unknown"
	}
}

// entQueryMetricEntities 返回预初始化 error counter 使用的固定实体集合。
func entQueryMetricEntities() []string {
	return []string{"permission", "role", "role_permission", "user", "user_role", "unknown"}
}
