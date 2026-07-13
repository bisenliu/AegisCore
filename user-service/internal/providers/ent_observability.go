package providers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/user-service/ent"
)

const (
	entQueryLatencyMetricName = "aegiscore_user_service_ent_query_duration_seconds"
	entQueryErrorMetricName   = "aegiscore_user_service_ent_query_errors_total"
	entQueryLatencyMetricHelp = "Ent query latency in seconds by fixed entity, query, and result."
	entQueryErrorMetricHelp   = "Total number of Ent query errors by fixed entity and query."

	entQueryOperation = "select"
	entResultSuccess  = "success"
	entResultError    = "error"
)

type entQueryMetrics struct {
	latency *prometheus.HistogramVec
	errors  *prometheus.CounterVec
}

func installEntObservability(client *ent.Client, metricsProvider *commonmetrics.Provider, tracingProvider *commontracing.Provider) error {
	metrics, err := newEntQueryMetrics(metricsProvider)
	if err != nil {
		return err
	}
	tracer := noop.NewTracerProvider().Tracer("github.com/aegiscore/user-service/ent")
	if tracingProvider != nil {
		tracer = tracingProvider.Tracer("github.com/aegiscore/user-service/ent")
	}
	installEntQueryObservability(client, tracer, metrics)
	return nil
}

func newEntQueryMetrics(provider *commonmetrics.Provider) (*entQueryMetrics, error) {
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

func installEntQueryObservability(client *ent.Client, tracer trace.Tracer, metrics *entQueryMetrics) {
	// Ent interceptor 只覆盖 query/select 路径，不覆盖 mutation；固定 entity/query/result label 用于控制指标基数。
	if client == nil {
		return
	}
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("github.com/aegiscore/user-service/ent")
	}
	client.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
			entity := entQueryEntity(query)
			ctx, span := tracer.Start(ctx, "ent.query",
				trace.WithAttributes(
					attribute.String("db.system", "postgresql"),
					attribute.String("ent.entity", entity),
					attribute.String("ent.query", entQueryOperation),
				),
			)
			startedAt := time.Now()
			value, err := next.Query(ctx, query)
			result := entResultSuccess
			if err != nil {
				result = entResultError
				span.RecordError(err)
				span.SetStatus(codes.Error, "ent query failed")
			}
			if metrics != nil {
				metrics.latency.WithLabelValues(entity, entQueryOperation, result).Observe(time.Since(startedAt).Seconds())
				if err != nil {
					metrics.errors.WithLabelValues(entity, entQueryOperation).Inc()
				}
			}
			span.End()
			return value, err
		})
	}))
}

func entQueryEntity(query ent.Query) string {
	// 通过 query 类型名映射低基数实体标签；新增 Ent schema 后需同步这里和 entQueryMetricEntities，否则会落到 unknown。
	if query == nil {
		return "unknown"
	}
	typeName := reflect.TypeOf(query).String()
	if idx := strings.LastIndex(typeName, "."); idx >= 0 {
		typeName = typeName[idx+1:]
	}
	typeName = strings.TrimPrefix(typeName, "*")
	typeName = strings.TrimSuffix(typeName, "Query")
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

func entQueryMetricEntities() []string {
	return []string{"permission", "role", "role_permission", "user", "user_role", "unknown"}
}
