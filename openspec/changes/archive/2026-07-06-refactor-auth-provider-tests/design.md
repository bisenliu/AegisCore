## Context

当前认证与 provider 测试已经完成部分现代化：auth/permission 的 `Metrics` 空实现由 `common/runtime/observability/metrics/nopgen` 生成，多个 application、transport、middleware 包已通过 `go.uber.org/mock/mockgen` 维护 mock。残留问题集中在测试文件组织与少量复杂测试替身：`session_store_test.go`、`service_test.go`、`routes_test.go` 和 `gin_test.go` 仍是大文件，且部分文件跨越多个测试子主题；provider、cmd、auth fx 和 Redis session store 仍有本地 fake/stub/spy。

本变更只调整测试与生成约定，不改变生产代码行为、HTTP API、数据库 schema、OpenAPI 生成物、部署清单、Prometheus metric family、OpenTelemetry tracing 或安全边界。

## Goals / Non-Goals

**Goals:**

- 将 auth Redis session store 测试按 token version、refresh session、purge pool 和 key schema 拆分。
- 将 auth command service 测试按 login、change-password、refresh、logout 和 shared helper 拆分。
- 将 provider routes 与 Gin engine 测试按 auth middleware、route conflict、tracing、request ID、metrics、panic 和 shared helper 拆分。
- 将复杂外部 collaborator 的测试替身迁移到现有或新增的 `mockgen` 生成物。
- 保留真实 Redis/miniredis、真实 `localcache`、领域值构造 helper 和无行为分支的轻量 fixture。
- 统一 metrics no-op 生成约定，继续使用业务中立的 `nopgen`。

**Non-Goals:**

- 不修改认证、授权、用户、角色、权限或 provider 的生产行为。
- 不新增 `common` 级业务 metrics interface，也不把 user-service 的业务指标方法移入 `common`。
- 不引入全局 `mocks/` 包或跨 feature mock 包。
- 不为了测试重构新增生产代码分支、`NewXForTest`、test hook 或额外 adapter。
- 不保留旧大文件布局与新拆分布局并行维护。

## Decisions

1. 按主题拆分测试文件，而不是只移动 helper。

   理由：当前维护成本主要来自单文件混合多个行为域，单纯移动 helper 不能改善定位和并行评审。拆分后每个文件围绕一类行为组织，测试函数名和文件名共同表达意图。

   备选方案：仅在文件中添加分段注释。该方案不改变冲突面，也不能减少审阅时的上下文噪声，因此不采用。

2. 使用包内 `mockgen` 生成物表达复杂 collaborator 交互。

   理由：端口调用顺序、失败注入、参数归一化和指标记录需要通过 expectation 明确表达。手写 fake/stub 容易隐藏调用路径，并在接口扩展时产生静默兼容。

   备选方案：继续保留本地 fake/stub 并补充字段。该方案会延续手写维护成本，与当前 mockgen 默认路径不一致，因此不采用。

3. 保留轻量测试 fixture，但限制其职责。

   理由：真实 Redis/miniredis、真实 `localcache`、领域值构造和无行为分支的 stats source 比 mock 更接近被测语义。它们不是外部 collaborator 契约替身，不需要强制 mockgen。

   备选方案：所有测试依赖全部 mockgen。该方案会降低集成语义测试价值，并为简单值对象制造不必要样板，因此不采用。

4. 继续使用 `nopgen`，只统一约定，不上提业务指标接口。

   理由：`common/runtime/observability/metrics/nopgen` 已经是业务中立生成器，能生成 feature-local no-op 实现。把 auth/permission 指标方法上提到 common 会违反 common 不承载 user-service 业务语义的边界。

   备选方案：创建 common 级统一 `NopMetrics` interface 或业务指标集合。该方案会污染 common 边界，因此不采用。

5. 以删除旧布局为迁移策略，不做兼容双轨。

   理由：测试文件名和测试替身不是外部运行时契约。保留旧文件或旧 fake/stub 只会让后续维护者面对两套路径。

   备选方案：先复制新文件再逐步停用旧文件。该方案会短期产生重复测试和冲突，不采用。

## Risks / Trade-offs

- 测试拆分时遗漏测试函数或 helper → 迁移前后运行目标包测试，并用 `rg "^func Test"` 对比测试函数集合。
- mock expectation 过度约束实现细节 → expectation 只覆盖外部端口契约、关键参数、调用顺序和安全/指标语义，不断言无业务意义的内部局部变量。
- helper 移动导致循环依赖或未使用导入 → 每个包迁移后立即运行对应 `go test`，最后运行 `make test`。
- 生成物 drift 未纳入提交 → 执行仓库约定生成命令，并在最终 `make verify` 前暂存本次预期变更。
- 误把业务 metrics 方法移入 common → 仅保留 `nopgen` 生成器和 feature-local `Metrics` interface，运行 `make user-service-architecture-lint` 检查边界。

## Migration Plan

1. 记录四个大测试文件当前测试函数清单，作为拆分完整性基线。
2. 为缺少生成 mock 的复杂 collaborator 增加包内 `mock_generate.go` 或扩展现有生成入口，并重新生成 mock 文件。
3. 按主题拆分 `session_store_test.go`，删除旧大文件中的重复内容。
4. 按 use case 拆分 `service_test.go`，共享构造 helper 移到 `service_test_helpers_test.go`。
5. 按 provider 子主题拆分 `routes_test.go` 和 `gin_test.go`，共享 provider helper 移到主题内 helper 文件。
6. 删除已被 mockgen 或主题 helper 替代的复杂手写 fake/stub/spy。
7. 运行目标包测试、生成命令、架构 lint、`make test`，最终按 OpenSpec 流程暂存预期变更后运行 `make lint` 和 `make verify`。

回滚方式：该变更只涉及测试和生成物，可通过版本控制回退本 change 的测试文件、mock 生成物和 OpenSpec artifact。由于不修改生产行为、数据结构或部署资产，无需运行时兼容或数据回滚。

## Open Questions

无。
