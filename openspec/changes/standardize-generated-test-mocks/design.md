## Context

当前仓库已经具备 feature-first 分层和 `common` 业务中立边界，但测试辅助对象和 metrics no-op 实现仍以手写为主。`auth/application/metrics.go` 与 `permission/application/metrics.go` 维护相似的空实现模式，多个 application、transport、infrastructure 测试文件维护包内 stub、fake 和 spy。

本 change 是跨 `common`、`user-service` 和交付验证流程的一次性不兼容调整。它不改变 HTTP API、数据库 schema、OpenAPI 生成物、部署清单、RBAC policy sync 行为、认证会话行为、Prometheus 指标名称或安全边界，但会改变 Go 测试代码组织、生成物管理和工具依赖。

## Goals / Non-Goals

**Goals:**

- 使用 `go.uber.org/mock/mockgen` 统一生成高重复 interface mock，替代大部分手写 stub、fake 和 spy。
- 保持 mock 生成物在接口消费侧 feature-local 测试包内，避免中央 mock 仓库破坏边界。
- 将 metrics no-op 实现改为统一生成，减少 `NopMetrics()` 模式重复。
- 确保 `common/runtime/observability/metrics` 只承载业务中立的生成能力或规范，不承载 user-service 业务指标接口。
- 将 mock 与 metrics no-op 生成物纳入 `go generate ./...` 和 drift 校验。

**Non-Goals:**

- 不把 auth、permission、role 或 user 的业务 interface 移入 `common`。
- 不创建全局 `mocks/`、`testmocks/` 或中央 mock 仓库。
- 不为了测试改造生产业务逻辑、HTTP handler、Ent schema、RBAC policy 或 runtime 配置语义。
- 不改变 Prometheus metric family、label key、label value 或 tracing/logging 运行时语义。
- 不兼容旧手写 stub/fake/spy 命名和辅助构造函数。

## Decisions

### Decision: mock 生成物放在 feature-local 测试包

`mockgen` 生成物放在接口消费侧测试包附近，例如 `user-service/internal/features/auth/application/command/mock_test.go`、`user-service/internal/features/role/application/command/mock_test.go`。生成物不得放入仓库根 `mocks/`、`common/mocks/` 或 `user-service/testmocks/`。

选择原因：application ports 由消费侧拥有，feature-local mock 能保持依赖方向清晰，避免跨 feature 测试细节泄漏。

备选方案：建立中央 mock 仓库。该方案被拒绝，因为它会鼓励测试跨 feature 复用内部接口 mock，弱化当前 feature-first 边界。

### Decision: 只生成高重复 interface mock，保留少量状态型测试 harness

高重复 store、session store、policy engine、notifier、metrics recorder 等 interface 使用 `mockgen` 生成。极少数需要复杂内存状态或 E2E 语义的测试辅助对象可以保留，但应使用 `testHarness`、`testStore` 或 `recordingMetrics` 等明确名称，不继续扩散 `Stub/Fake/Spy` 命名模式。

选择原因：全量 mock 化会让需要状态模拟的测试变得脆弱；保留少量测试 harness 可以避免为了 mock 而牺牲可读性。

备选方案：所有测试 double 全部替换为 gomock。该方案被拒绝，因为部分测试需要模拟 Redis、健康状态或批量 seed 的状态演进，纯 expectation mock 会增加样板和维护成本。

### Decision: metrics no-op 采用生成化，不下沉业务 interface

`auth/application` 与 `permission/application` 继续拥有自己的 `Metrics` interface 和 `NopMetrics()` 入口。手写 `nopMetrics` 方法实现迁移为生成文件，例如 `metrics_nop_gen.go`。`common/runtime/observability/metrics` 只提供业务中立的生成工具或生成规范。

选择原因：两个 feature 的 metrics 方法集不同，单一通用 no-op 类型无法直接实现任意业务 interface；将业务 interface 下沉到 `common` 会让共享模块承载服务业务语义。

备选方案：在 `common/runtime/observability/metrics` 定义 auth/permission metrics interface。该方案被拒绝，因为它违反 `common` 只承载跨服务稳定能力和无业务语义 primitive 的边界。

### Decision: 生成物 drift 纳入交付验证

实现后 `go generate ./...` 必须覆盖 mock 与 metrics no-op 生成物，并通过 `git diff --exit-code` 暴露生成物 drift。`make verify` 或等价验证链路需要包含该检查。

选择原因：生成物如果未进入统一验证，会重新形成手写漂移或过期 mock。

备选方案：仅要求开发者手动运行生成命令。该方案被拒绝，因为它无法在 CI 或本地完整验证中稳定发现 drift。

## Risks / Trade-offs

- [Risk] `gomock` expectation 过度精确导致测试更脆弱 → Mitigation: 只在需要交互断言的高重复 interface 使用 mock，状态型流程保留命名清晰的测试 harness。
- [Risk] 生成工具引入后不同模块工具依赖不一致 → Mitigation: 在涉及模块显式声明 `mockgen` 工具入口，并通过 `go generate ./...` 验证。
- [Risk] metrics no-op generator 错误生成业务方法签名 → Mitigation: 生成物必须编译通过，并由 feature package 测试覆盖 `NopMetrics()` 赋值给对应 `Metrics` interface。
- [Risk] 全局 mock 目录后续被重新引入 → Mitigation: 在 spec 和架构 lint 任务中明确禁止中央 mock 仓库。
- [Risk] 一次性删除手写 stub/fake/spy 导致测试迁移 diff 较大 → Mitigation: 按 package 依赖顺序迁移并逐包运行测试，最终再执行 `make lint` 和 `make verify`。

## Migration Plan

1. 在 OpenSpec delta 和任务中固定生成规范、放置规则和 drift 校验要求。
2. 引入 `mockgen` 工具入口和 metrics no-op 生成工具或生成规范。
3. 先迁移 `auth/application/command`、`role/application/command`、`role/application/seed` 等高重复测试 double。
4. 迁移 `permission`、`user` 和必要的 `common` 测试 double。
5. 将 `auth` 与 `permission` 的手写 no-op metrics 迁移为生成文件。
6. 运行 `go generate ./...`、相关包测试、`make lint`、`make user-service-architecture-lint` 和 `make verify`。

回滚方式：在未发布前通过 git revert 回滚本 change 的生成工具、生成物和测试迁移提交。由于不涉及数据库、HTTP API 或部署资产，回滚不需要数据迁移或运行时兼容步骤。

## Open Questions

- 无。当前 change 按一次性不兼容迁移执行，不保留旧手写 stub/fake/spy 兼容层。
