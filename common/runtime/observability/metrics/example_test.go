package metrics_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/observability/metrics"
)

// ExampleProvider_Register 展示启用状态 provider 的本地 registry、collector 注册、
// 重复注册幂等和 GatherContext 采集用法。
func ExampleProvider_Register() {
	// 每个 Provider 都拥有自己的 Prometheus registry。示例使用本地 provider，
	// 避免把 collector 注册到 Prometheus global registry，从而保证测试和多个
	// runtime App 并行构造时互不污染。
	provider, _ := metrics.NewProvider(metrics.Options{
		Config:      config.MetricsConfig{Enabled: true},
		ServiceName: "example-service",
		Environment: "local",
	})

	// 自定义 collector 仍然使用标准 prometheus collector 类型。Provider.Register
	// 会统一包装 service/environment label，并把重复注册视为成功，便于不同 wiring
	// 路径安全地注册同一稳定 collector。
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_example_requests_total",
		Help: "Example request count.",
	})

	_ = provider.Register(counter)
	_ = provider.Register(counter)
	counter.Inc()

	// GatherContext 用调用方提供的 context 采集支持 ContextCollector 的 collector。
	// 对普通 prometheus.Collector 来说，它与本地 registry 的 Gather 行为保持一致。
	families, _ := provider.GatherContext(context.Background())

	// 示例只判断稳定的 metric family 是否存在，不断言完整 Prometheus 文本格式，
	// 避免把示例绑定到与本契约无关的序列化细节。
	found := false
	for _, family := range families {
		if family.GetName() == "aegiscore_example_requests_total" {
			found = true
			break
		}
	}
	fmt.Println(provider.Enabled())
	fmt.Println(found)
	// Output:
	// true
	// true
}

// ExampleProvider_HTTPHandler 展示如何用 Provider.HTTPHandler 暴露基于 HTTP
// request context 的 Prometheus scrape handler。
func ExampleProvider_HTTPHandler() {
	// HTTPHandler 是运行时 metrics endpoint 的推荐入口。它内部使用 request
	// context 调用 GatherContext，使 Redis PING 等 context-aware collector 可以在
	// scrape 取消时尽快返回。
	provider, _ := metrics.NewProvider(metrics.Options{
		Config:      config.MetricsConfig{Enabled: true},
		ServiceName: "example-service",
		Environment: "local",
	})

	// 示例 collector 只使用本地内存状态，不访问公网或真实 datastore，确保 go test
	// 执行示例时稳定、快速且没有外部依赖。
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aegiscore_example_queue_depth",
		Help: "Example queue depth.",
	})
	_ = provider.Register(gauge)
	gauge.Set(3)

	// httptest 模拟一次 scrape 请求。实际服务会把返回的 handler 挂载到配置化
	// metrics endpoint；示例只验证 handler 能输出已注册 collector。
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	provider.HTTPHandler(promhttp.HandlerOpts{}).ServeHTTP(recorder, request)

	fmt.Println(recorder.Code)
	fmt.Println(strings.Contains(recorder.Body.String(), "aegiscore_example_queue_depth"))
	// Output:
	// 200
	// true
}

// ExampleProvider_disabled 展示禁用状态 provider 的 no-op 注册、空 gather 结果和
// HTTPHandler 不暴露 metrics 内容的行为。
func ExampleProvider_disabled() {
	// 禁用 metrics 时仍然返回非 nil Provider，以满足正式依赖图中非 optional 的
	// metrics 依赖；但该 provider 不暴露 registerer、gatherer 或 metrics 输出。
	provider, _ := metrics.NewProvider(metrics.Options{
		Config:      config.MetricsConfig{Enabled: false},
		ServiceName: "example-service",
		Environment: "local",
	})

	// disabled provider 上的 Register 是 no-op，包括 nil collector 在内的注册请求
	// 都不会触发 Prometheus side effect。这里使用普通 counter 展示调用方无需额外
	// 分支即可安全执行注册路径。
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegiscore_disabled_example_total",
		Help: "Disabled example count.",
	})
	_ = provider.Register(counter)

	// GatherContext 在 disabled provider 上返回 nil slice；HTTPHandler 返回 404，
	// 表示服务不应暴露 metrics 内容或空的 scrape 成功响应。
	families, _ := provider.GatherContext(context.Background())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	provider.HTTPHandler(promhttp.HandlerOpts{}).ServeHTTP(recorder, request)

	fmt.Println(provider.Enabled())
	fmt.Println(len(families))
	fmt.Println(recorder.Code)
	// Output:
	// false
	// 0
	// 404
}
