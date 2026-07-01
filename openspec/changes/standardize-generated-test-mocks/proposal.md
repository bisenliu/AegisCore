## Why

当前测试 double 与 metrics no-op 实现以手写为主，已在多个 feature 中形成重复模式：`auth` 与 `permission` 分别维护近似的 `NopMetrics()` 空实现，测试文件中也存在多处手写 stub、fake 和 spy。需要一次性建立生成化规范，减少重复维护成本，并避免继续扩散不一致的手写测试辅助对象。

本 change 接受一次性不兼容调整：不保留旧 stub/fake/spy 兼容层，不把服务业务指标接口下沉到 `common`，而是通过统一生成规范保持 feature 边界和生成物可校验。

## What Changes

- **BREAKING** 删除高重复手写 stub/fake/spy，改用 `go.uber.org/mock` 生成的 feature-local mock。
- **BREAKING** 删除 `auth/application` 与 `permission/application` 中手写 `nopMetrics` 方法实现，改由统一 metrics no-op 生成流程产出生成文件。
- 引入 `go.uber.org/mock/mockgen` 作为标准 mock 生成工具，并为需要生成 mock 的 package 增加 `go:generate` 入口。
- 在 `common/runtime/observability/metrics` 边界提供业务中立的 metrics no-op 生成能力或生成规范，不承载 user-service 的 auth、permission、role 或 user 业务方法。
- mock 生成物放在接口消费侧 feature-local 测试包内，不创建全局 `mocks/` 或中央 mock 仓库。
- 交付验证流程增加生成物 drift 检查，确保 `go generate ./...` 后不会产生未提交 diff。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 增加跨模块测试生成规范，约束共享测试基础设施、mock 生成工具和 feature-local mock 归属。
- `runtime-observability`: 增加 metrics no-op 生成规范，确保业务 metrics 接口保留在所属 feature，同时通过业务中立工具生成空实现。
- `delivery-operations`: 增加 Go 生成物校验要求，确保 mock 与 metrics no-op 生成物纳入 `go generate` 和 drift 检查。

## Impact

- 影响 Go 测试代码：`user-service/internal/features/auth|permission|role|user/**` 与部分 `common/**` 测试中的高重复手写 test double 将被替换为生成 mock。
- 影响 Go 生产代码：`auth/application/metrics.go`、`permission/application/metrics.go` 中手写 no-op 实现将迁移到生成文件；feature-local `Metrics` interface 与 `NopMetrics()` 调用语义保持不变。
- 影响共享契约：`common/runtime/observability/metrics` 只新增业务中立生成能力或规范，不引入 user-service 业务语义。
- 影响依赖：`go.uber.org/mock/mockgen` 需要成为显式 tool 依赖或工具入口。
- 影响交付验证：`go generate ./...` 与 `git diff --exit-code` 需要覆盖 mock 和 metrics no-op 生成物 drift。
- 不影响 HTTP API、数据库 schema、OpenAPI 输出、部署清单、RBAC 授权语义、认证会话语义或 Prometheus 指标名称与标签语义。
