// Package metrics 为 AegisCore runtime component 提供业务中立的 Prometheus
// metrics primitive。
//
// Provider 始终为依赖图提供非 nil 值。metrics 禁用时，provider 是 no-op：
// Enabled 返回 false，Register 和 MustRegister 无副作用，Registerer 和 Gatherer
// 返回 nil。metrics 启用时，provider 拥有独立 Prometheus registry，并通过稳定
// service 和 environment label 包装 registerer。provider 不向 Prometheus global
// registry 安装 collector。
//
// Register 将 Prometheus 重复注册视为成功，使独立 runtime wiring 可以安全重复
// 注册同一稳定 collector。其他注册错误会返回；启用 provider 会拒绝 nil collector。
//
// HTTPHandler 暴露 promhttp handler，并使用当前 HTTP request context 调用
// GatherContext。实现 ContextCollector 的 collector 可以使用该 context 执行 Redis
// PING 等 scrape-scoped work。直接通过 Gatherer().Gather 或标准
// prometheus.Collector Collect 方法采集时不携带 HTTP request context，因此使用
// background context 和 collector 自身 timeout 行为。
//
// 本包的 runtime collector 和 adapter 必须保持 label 低基数。label 可以包含固定
// resource name，以及枚举 result、state、event 或 reason。label 不得包含主体标识、
// 访问控制实体、会话凭据、trace/span ID、raw path、IP、email、username、SQL、Redis
// key、原始错误文本或服务私有业务状态。
package metrics
