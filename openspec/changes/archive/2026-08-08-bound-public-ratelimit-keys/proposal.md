## Why

当前公网匿名限流只限制单个 Key 的请求速率，没有限制可创建的新 Key 总量。攻击者通过大量源 IP 或 IPv6 地址轮换访问公开认证入口时，会让进程内 `visitors` map 线性增长，触发堆内存、GC 压力和最终 OOM，使限流器本身成为内存放大器。

该问题属于安全与性能阻断项，需要在上线前为匿名限流状态建立硬上界，并保证容量耗尽时的行为可预测、可观测。

## What Changes

- 为 API 限流器增加全局或分片级最大 Key 容量约束，使活跃 limiter 条目数不超过配置上限。
- 在容量耗尽时提供明确的降级策略：优先复用共享 overflow bucket 或拒绝新 Key，并保留已有 Key 的限流状态，避免通过持续制造新 Key 绕过限流。
- 增加容量、当前 Key 数、驱逐、overflow 或拒绝等可观测指标，支持安全运营和压测验收。
- 扩展 user-service 限流配置，允许服务为公网匿名入口设置容量上限、降级策略和合理默认值。
- 增加唯一 Key 压测/单元测试/race 测试覆盖，验证条目数有界、已有 Key 状态不被绕过。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `api-rate-limiting`: 匿名和认证限流器必须对进程内 Key 状态设置容量上限，并定义容量耗尽时的安全降级和观测行为。

## Impact

- 影响 `common/http/middleware/ratelimit.go` 的限流器状态管理、清理逻辑和指标暴露。
- 影响 `user-service/internal/config/config.go` 的限流配置模型、默认值和校验。
- 影响 `user-service/internal/router/router.go` 或相关 provider 的限流器构造参数。
- 不改变公开 HTTP API 路径、请求/响应 DTO 或数据库 schema。
- 可能新增 Prometheus 指标或扩展既有限流指标；部署观测资产如引用这些指标需同步更新。
