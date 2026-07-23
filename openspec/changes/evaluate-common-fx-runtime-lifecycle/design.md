## Context

当前 `common/runtime/observability/metrics/fx.go` 和 `common/runtime/observability/tracing/fx.go` 暴露的 `NewFxProvider` 名称强调 Fx 绑定，但 metrics 和 tracing 本身是跨服务 runtime observability 能力。`common/runtime/timezone/fx.go` 仅包装 `Init`，其价值更接近 process runtime 初始化，而不是独立资源 provider。user-service 的 Fx composition root 需要清晰表达 process runtime 初始化、observability provider、业务 lifecycle module 和 runtime server 注册之间的顺序关系。

该变更横跨 `common` 与 `user-service`，但不改变 HTTP API、数据库结构、OpenAPI、Prometheus 指标契约、OpenTelemetry 配置或部署资产。主要协作者是维护 common runtime primitive、user-service composition root 和架构边界验证的开发者。

## Goals / Non-Goals

**Goals:**

- 让 common observability 的公开 provider 名称表达具体 runtime 能力，避免多个包重复暴露含义模糊的 `NewFxProvider`。
- 让 timezone 初始化由拥有进程启动语义的服务 composition root 显式绑定，或保留一个具有明确 runtime 语义的入口。
- 保持 metrics/tracing 的 constructor 阶段语义、Fx lifecycle 启停语义和 no-op 行为不变。
- 保持 user-service 的 App 构图、启动、停止、测试和架构 lint 可验证。

**Non-Goals:**

- 不新增 metrics family、tracing span、日志字段、健康检查或部署观测资产。
- 不修改 runtime config 字段、环境变量、资源名称、PostgreSQL/Redis 初始化或 lifecycle stop budget 计算。
- 不引入新的 DI framework abstraction，也不把 user-service 私有配置、feature provider 或业务 lifecycle 放入 `common`。
- 不调整 HTTP route、OpenAPI 注解、Ent schema、Atlas migration 或 RBAC policy sync。

## Decisions

- 将 common observability provider 的公开命名收敛为能力名优先的入口，例如 metrics package 暴露 metrics provider 构造入口，tracing package 暴露 tracing provider 构造入口；调用方通过 import alias 或服务级 module 命名表达上下文。备选方案是保留 `NewFxProvider`，但该名称在多个包中重复出现，无法从调用点直接区分能力，继续强化 common API 与 Fx 的耦合感。
- timezone 初始化不再作为仅包装 `Init` 的 common Fx provider 优先暴露；服务 composition root 应显式 `fx.Invoke` timezone 初始化或调用一个语义清晰的 service-level process runtime 初始化函数。备选方案是在 common 中保留 timezone Fx module，但这会把进程初始化决策隐藏在 common adapter 中，降低 App 启动顺序的可读性。
- user-service 在 `internal/bootstrap` 或 `internal/providers` 中维护服务级 runtime module 名称和顺序，先绑定 process runtime 初始化，再装配 common observability provider、服务资源 provider、feature lifecycle module 和 runtime server invoke。备选方案是继续让 common 包提供更大的 runtime module，但这会使 common 依赖服务装配语义或诱导跨服务共享不稳定组合。
- 所有更名必须同步更新直接调用点和测试，不新增兼容别名，除非发现外部消费者或生成物依赖当前 API。当前仓库是 workspace 内部消费，最小正确变更优先于长期兼容层。

## Risks / Trade-offs

- `common` 公开函数重命名可能导致 workspace 内未覆盖调用点编译失败。缓解方式：使用 `go test` 或相关 package 测试覆盖 `common/runtime/observability/...` 与 `user-service/internal/...`，并运行架构 lint。
- 移除或弱化 timezone Fx wrapper 可能改变 `Init` 执行时机。缓解方式：在 user-service composition root 中显式 `fx.Invoke`，并通过 App 构图或启动测试验证初始化仍发生在 runtime server 启动前。
- provider 命名调整可能只改善可读性但不改变行为。缓解方式：spec delta 聚焦长期边界和可验证调用关系，tasks 中要求验证无 API、schema、OpenAPI 和观测资产 drift。
- 不保留兼容别名会让未来新增服务需要使用新命名。缓解方式：该仓库当前没有外部 Go API 发布要求，OpenSpec 明确不新增兼容层。

## Migration Plan

- 修改 common provider 命名和 timezone 初始化入口归属。
- 更新 user-service composition root 中的 Fx option/module 调用点。
- 更新受影响测试或架构断言，使其验证新命名和显式 lifecycle 绑定。
- 回滚时恢复原函数名、timezone Fx wrapper 和 user-service module 绑定即可；该变更不涉及数据迁移、部署清单迁移或外部契约迁移。

## Open Questions

- 无待决问题；实现阶段如发现当前 `NewFxProvider` 已被仓库外 Go module 消费，需要重新评估是否保留已弃用兼容入口。
