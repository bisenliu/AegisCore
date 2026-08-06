// Package tracing 为 AegisCore runtime component 提供业务中立的 OpenTelemetry
// tracing facade。
//
// NewProvider 会立即构造并启动 provider。调用方拥有返回的 provider，并必须在不再使用时
// 调用 Shutdown。NewTracingProvider 是 Fx adapter：它在依赖图构造阶段返回可注入的
// facade，在 Fx OnStart hook 中启动 SDK provider，并在 OnStop 或 Fx rollback 中关闭。
//
// Provider 或 OTelTracerProvider 返回的 tracer 是动态 tracer，constructor 阶段的
// instrumentation 可以安全保存。启动前和 Shutdown 后它们委托 no-op provider，启动后委托
// 当前活跃的 SDK provider。本包不安装 OpenTelemetry global provider 或 propagator。
package tracing
