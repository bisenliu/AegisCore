package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	platformroute "github.com/aegiscore/platform/http/route"
	platformmetrics "github.com/aegiscore/platform/runtime/observability/metrics"
)

const (
	// unknownHTTPMetricsMethod 是无法识别请求方法时记录在指标中的占位方法名。
	unknownHTTPMetricsMethod = "UNKNOWN"

	// httpServerRequestsMetricName 是 HTTP 请求总数 Counter 指标名。
	httpServerRequestsMetricName = "http_server_requests_total"
	// httpServerDurationMetricName 是 HTTP 请求耗时 Histogram 指标名（单位：秒）。
	httpServerDurationMetricName = "http_server_request_duration_seconds"
	// httpServerInFlightRequestsName 是当前进行中 HTTP 请求数 Gauge 指标名。
	httpServerInFlightRequestsName = "http_server_in_flight_requests"
	// httpServerRequestsMetricHelp 是 HTTP 请求总数指标的帮助说明。
	httpServerRequestsMetricHelp = "Total number of completed HTTP server requests."
	// httpServerDurationMetricHelp 是 HTTP 请求耗时指标的帮助说明。
	httpServerDurationMetricHelp = "Duration of completed HTTP server requests in seconds."
	// httpServerInFlightRequestsHelp 是进行中请求数指标的帮助说明。
	httpServerInFlightRequestsHelp = "Current number of in-flight HTTP server requests."
)

// HTTPMetricsOptions 配置 HTTP server RED 指标中间件。
type HTTPMetricsOptions struct {
	Provider        *platformmetrics.Provider
	Skip            func(*gin.Context) bool
	SkipResult      func(*gin.Context) bool
	DurationBuckets []float64
}

type httpServerMetricsRecorder struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight *prometheus.GaugeVec
}

// HTTPServerMetrics 记录 Gin HTTP 入站请求的 RED 指标。
func HTTPServerMetrics(options HTTPMetricsOptions) gin.HandlerFunc {
	recorder := newHTTPServerMetricsRecorder(options)
	if recorder == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		if options.Skip != nil && options.Skip(c) {
			c.Next()
			return
		}

		method := normalizeHTTPMetricsMethod(c.Request.Method)
		inFlightRoute := platformroute.TemplateOrUnmatched(c)
		start := time.Now()

		recorder.inFlight.WithLabelValues(method, inFlightRoute).Inc()
		defer func() {
			status := c.Writer.Status()
			// decrement 必须复用 increment 时捕获的路由标签；若请求结束后重新取值，标签变化会让原 gauge 无法归零。
			recorder.inFlight.WithLabelValues(method, inFlightRoute).Dec()

			if options.SkipResult != nil && options.SkipResult(c) {
				return
			}

			route := platformroute.TemplateOrUnmatched(c)
			statusClass := platformmetrics.StatusClass(status)
			recorder.requests.WithLabelValues(method, route, statusClass).Inc()
			recorder.duration.WithLabelValues(method, route, statusClass).Observe(time.Since(start).Seconds())
		}()

		c.Next()
	}
}

// newHTTPServerMetricsRecorder 创建接口访问统计组件，用于记录系统一共收到了多少次请求以及处理耗时。
func newHTTPServerMetricsRecorder(options HTTPMetricsOptions) *httpServerMetricsRecorder {
	if options.Provider == nil || !options.Provider.Enabled() {
		return nil
	}

	buckets := options.DurationBuckets
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}

	recorder := &httpServerMetricsRecorder{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: httpServerRequestsMetricName,
			Help: httpServerRequestsMetricHelp,
		}, []string{platformmetrics.LabelMethod, platformmetrics.LabelRoute, platformmetrics.LabelStatusClass}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    httpServerDurationMetricName,
			Help:    httpServerDurationMetricHelp,
			Buckets: buckets,
		}, []string{platformmetrics.LabelMethod, platformmetrics.LabelRoute, platformmetrics.LabelStatusClass}),
		inFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: httpServerInFlightRequestsName,
			Help: httpServerInFlightRequestsHelp,
		}, []string{platformmetrics.LabelMethod, platformmetrics.LabelRoute}),
	}

	for _, collector := range []prometheus.Collector{recorder.requests, recorder.duration, recorder.inFlight} {
		if err := options.Provider.Register(collector); err != nil {
			panic(err)
		}
	}
	return recorder
}

// normalizeHTTPMetricsMethod 将请求方式转为标准大写字母（如 GET、POST）；对于不常见的非标准方式归为 OTHER，防止监控标签过多导致内存暴涨。
func normalizeHTTPMetricsMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet:
		return http.MethodGet
	case http.MethodHead:
		return http.MethodHead
	case http.MethodPost:
		return http.MethodPost
	case http.MethodPut:
		return http.MethodPut
	case http.MethodPatch:
		return http.MethodPatch
	case http.MethodDelete:
		return http.MethodDelete
	case http.MethodConnect:
		return http.MethodConnect
	case http.MethodOptions:
		return http.MethodOptions
	case http.MethodTrace:
		return http.MethodTrace
	default:
		return unknownHTTPMetricsMethod
	}
}
