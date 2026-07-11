package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
)

const (
	defaultHTTPMetricsRouteFallback = "__unmatched__"
	unknownHTTPMetricsMethod        = "UNKNOWN"

	httpServerRequestsMetricName   = "http_server_requests_total"
	httpServerDurationMetricName   = "http_server_request_duration_seconds"
	httpServerInFlightRequestsName = "http_server_in_flight_requests"
	httpServerRequestsMetricHelp   = "Total number of completed HTTP server requests."
	httpServerDurationMetricHelp   = "Duration of completed HTTP server requests in seconds."
	httpServerInFlightRequestsHelp = "Current number of in-flight HTTP server requests."
)

// HTTPMetricsOptions 配置 HTTP server RED 指标中间件。
type HTTPMetricsOptions struct {
	Provider        *commonmetrics.Provider
	Skip            func(*gin.Context) bool
	SkipResult      func(*gin.Context) bool
	RouteFallback   string
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

	routeFallback := normalizeRouteFallback(options.RouteFallback)
	return func(c *gin.Context) {
		if options.Skip != nil && options.Skip(c) {
			c.Next()
			return
		}

		method := normalizeHTTPMetricsMethod(c.Request.Method)
		inFlightRoute := routeTemplateOrFallback(c, routeFallback)
		start := time.Now()

		recorder.inFlight.WithLabelValues(method, inFlightRoute).Inc()
		defer func() {
			status := c.Writer.Status()
			recorder.inFlight.WithLabelValues(method, inFlightRoute).Dec()

			if options.SkipResult != nil && options.SkipResult(c) {
				return
			}

			route := routeTemplateOrFallback(c, routeFallback)
			statusClass := commonmetrics.StatusClass(status)
			recorder.requests.WithLabelValues(method, route, statusClass).Inc()
			recorder.duration.WithLabelValues(method, route, statusClass).Observe(time.Since(start).Seconds())
		}()

		c.Next()
	}
}

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
		}, []string{commonmetrics.LabelMethod, commonmetrics.LabelRoute, commonmetrics.LabelStatusClass}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    httpServerDurationMetricName,
			Help:    httpServerDurationMetricHelp,
			Buckets: buckets,
		}, []string{commonmetrics.LabelMethod, commonmetrics.LabelRoute, commonmetrics.LabelStatusClass}),
		inFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: httpServerInFlightRequestsName,
			Help: httpServerInFlightRequestsHelp,
		}, []string{commonmetrics.LabelMethod, commonmetrics.LabelRoute}),
	}

	for _, collector := range []prometheus.Collector{recorder.requests, recorder.duration, recorder.inFlight} {
		if err := options.Provider.Register(collector); err != nil {
			panic(err)
		}
	}
	return recorder
}

func normalizeRouteFallback(routeFallback string) string {
	routeFallback = strings.TrimSpace(routeFallback)
	if routeFallback == "" {
		return defaultHTTPMetricsRouteFallback
	}
	return routeFallback
}

func routeTemplateOrFallback(c *gin.Context, fallback string) string {
	if route := c.FullPath(); route != "" {
		return route
	}
	return fallback
}

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
