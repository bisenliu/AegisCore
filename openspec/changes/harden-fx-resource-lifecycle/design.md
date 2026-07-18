## Context

当前 runtime 与 user-service 的若干 Fx provider 在 constructor 阶段创建运行资源，然后通过 `fx.StopHook` 或 `fx.Hook.OnStop` 注册关闭逻辑。该模式在 App 成功 `Start` 后正常 `Stop` 时成立，但在 `fx.New` 已构造部分对象、后续 constructor 或 Invoke 失败，或者 `Start` 阶段后续 hook 失败时，不能把 stop hook 视为 constructor rollback。

本次 change 覆盖 `common/runtime/observability/tracing`、`common/runtime/datastore`、`user-service/internal/features/auth`、`user-service/internal/features/permission` 和 `user-service/internal/providers` 的资源生命周期接线。它不改变 HTTP API、数据库 schema、OpenAPI 生成物、部署清单或安全策略，只降低启动失败路径下的资源泄漏风险。

## Goals / Non-Goals

**Goals:**

- 将有后台副作用或长期连接语义的运行资源优先迁移为 `OnStart` 创建、`OnStop` 关闭，constructor 返回 holder、factory 或无后台副作用对象。
- 为 constructor 内必须创建的资源补齐部分失败即时清理，避免等待 Fx stop hook 兜底。
- 保持 auth、permission、runtime observability 和 datastore 的现有业务行为、配置名称、指标标签、tracing 语义和关闭顺序。
- 补充启动失败、停止幂等和部分构造失败的测试覆盖。

**Non-Goals:**

- 不迁移所有 constructor；`sql.Open` 等通常只创建轻量 pool handle 且无后台 goroutine 的对象可以保留现状。
- 不引入新的 DI 框架、全局资源 registry、业务共享包或跨 feature helper。
- 不改变 Ent schema、Atlas migration、OpenAPI 注解、HTTP route、RBAC policy 语义或部署拓扑。
- 不为测试便利新增无运行时职责的生产接口、全局可变函数或 `NewXForTest`。

## Decisions

1. 对主动运行资源采用 holder + lifecycle hook。

   运行资源包括 tracing batch processor/exporter、worker pool、带内部清理行为的 local cache、Redis client、Redis watcher、Ent observability wrapper 等。constructor 仅返回 holder 或完成静态校验，`OnStart` 创建真实资源并写入 holder，`OnStop` 读取 holder 并关闭。

   备选方案：保留 constructor 创建并依赖 `fx.StopHook`。该方案改动小，但无法覆盖后续构图失败时未进入正常启动的泄漏路径。

2. holder 暴露最小运行时方法，并在未启动或已关闭时返回明确错误或 no-op 语义。

   对正式请求路径需要的依赖，holder 必须在 `OnStart` 前不可被业务流量使用；若被调用，应按现有安全边界 fail-closed 或返回明确内部错误。对 metrics/tracing no-op 场景，保持现有非 nil 依赖和禁用语义。

   备选方案：让每个 consumer 自行持有 optional 指针。该方案会把 lifecycle 状态扩散到业务层，增加 nil 分支和安全退化风险。

3. 必须在 constructor 中创建的资源执行局部 rollback。

   如果某个 constructor 内连续创建多个部分资源，后续步骤失败时必须立即关闭已创建资源，并使用 `errors.Join` 或等价方式保留原始失败与清理失败。Redis 启动 PING 失败关闭 client 的既有模式应作为基准。

   备选方案：要求所有资源都延迟到 `OnStart`。该方案更统一，但会对轻量 handle 和部分框架包装造成过度改动。

4. 生命周期编排留在 composition 边界。

   `common` 只提供业务中立 runtime primitive 和 framework-adapter；auth、permission 的 infrastructure 正式代码不引入 Fx/Dig，feature `fx.go` 负责注册 `Initialize`、`Start`、`Stop`、`Close` 等显式方法。`user-service/internal/shared` 不承载资源 lifecycle helper。

   备选方案：抽取一个通用 managed resource helper。当前资源形态和错误语义差异较大，抽象会增加不必要的 API 面和跨模块耦合。

## Risks / Trade-offs

- [Risk] holder 在 `OnStart` 前被调用可能暴露 nil 资源或错误路径。→ 通过窄方法、mutex/atomic 状态和启动失败测试覆盖，确保返回明确错误或保持既有 no-op/fail-closed 语义。
- [Risk] 将创建延迟到 `OnStart` 会改变错误出现阶段。→ 保持错误内容可诊断，并在启动阶段传播失败，使 App 不进入 ready 状态。
- [Risk] 关闭顺序不当可能让 auth/permission 自有资源晚于共享 Redis 或 Ent 关闭。→ 只在 composition 层登记 hook，并通过 Fx 逆序规则与测试验证自有资源先关闭。
- [Risk] tracing 或 metrics 禁用语义被误改。→ 禁用模式必须继续提供非 nil no-op provider，且不得连接 exporter 或注册不应有的 collector。
- [Risk] 过度迁移轻量对象增加代码复杂度。→ 明确不迁移 `sql.Open` 等轻量 handle，优先处理主动后台资源和长期连接。

## Migration Plan

1. 更新相关 delta spec 后，在 `common` 与 user-service feature composition 中分批迁移高风险资源。
2. 先处理 tracing、worker pool、local cache、Redis watcher/client 和 Ent wrapper 的启动失败测试，再调整实现。
3. 保持配置、API、schema、OpenAPI 和部署资产不变；发布时按普通 user-service 镜像滚动发布即可。
4. 回滚策略为回退本次代码改动；由于无 schema/API 变更，不需要数据迁移回滚。

## Verification

- 运行相关包测试：`go test ./common/runtime/observability/tracing/...`、`go test ./common/runtime/datastore/...`、`go test ./user-service/internal/features/auth/...`、`go test ./user-service/internal/features/permission/...`、`go test ./user-service/internal/providers/...`。
- 运行架构检查：`make user-service-architecture-lint`。
- 合并前运行 `make lint` 和 `make verify`。

## Open Questions

- 无。
