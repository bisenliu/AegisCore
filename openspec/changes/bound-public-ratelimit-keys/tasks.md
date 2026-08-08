## 1. Common 限流器有界状态

- [x] 1.1 在 `common/http/middleware/ratelimit.go` 扩展 `LocalRateLimiterOptions`，加入 `MaxKeys` 和容量耗尽策略，并在构造函数中校验启用策略的容量与策略值。
- [x] 1.2 在 `LocalRateLimiter` 的分片状态中加入容量预算和共享 overflow limiter，确保新 Key 创建前检查容量，已有 Key 状态不被新 Key 重置。
- [x] 1.3 实现 overflow 与 reject 行为，返回稳定的容量耗尽 reason 或错误，供 Gin middleware 和 user-service 观测层消费。
- [x] 1.4 补充 `common/http/middleware` 单元测试，覆盖唯一 Key 超容量、已有 Key 继续限流、overflow bucket、reject、TTL 清理释放容量和并发访问。

## 2. User-service 配置和接线

- [x] 2.1 扩展 `user-service/internal/config/config.go` 的 `RateLimitPolicyConfig`，加入 `max_keys` 和容量耗尽策略配置、默认值和启动前校验。
- [x] 2.2 更新配置加载与默认值测试，验证缺省策略包含有限正数 `MaxKeys`，非法容量和非法策略会返回字段路径错误。
- [x] 2.3 更新 `user-service/internal/providers/transport/ratelimit.go`，把服务私有配置传递给 `commonmw.NewLocalRateLimiter`。
- [x] 2.4 更新 transport provider 测试，验证启用策略会构造带容量上界的 limiter，禁用策略不创建 janitor 或 limiter。

## 3. 观测与路由行为

- [x] 3.1 扩展 `user-service/internal/router/ratelimit_observability.go` 的固定 reason 映射，覆盖容量耗尽、overflow 和 reject，保持 metrics 标签低基数。
- [x] 3.2 补充 router 或 middleware 行为测试，验证容量耗尽时公开认证入口不会写入原始 IP/User ID 到 metrics 标签或日志字段。
- [x] 3.3 检查 deployments 观测资产是否引用限流指标；如需展示新增 reason，更新对应 Prometheus/Grafana 资产并运行 drift 检查。

## 4. 验证与交付

- [x] 4.1 运行 `go test ./common/http/middleware ./user-service/internal/config ./user-service/internal/providers/transport ./user-service/internal/router`。
- [x] 4.2 运行 `go test -race ./common/http/middleware`，验证并发容量控制无数据竞争。
- [x] 4.3 运行 `make user-service-architecture-lint`，验证 common、user-service 和 openspec 边界未被破坏。
- [x] 4.4 将本次预期代码、测试、OpenSpec 和必要观测资产变更加到暂存区。
- [x] 4.5 运行 `make lint`，失败时修复后重跑。
- [x] 4.6 运行 `make verify`，失败时修复后重跑，确保最终 drift 检查通过。
