## Context

`common/runtime/logger/fx.go` 当前只负责创建配置化 `*zap.Logger` 并在 Fx `OnStop` 阶段执行 `Sync`。`user-service/internal/bootstrap/app.go` 的 `AppOptions` 已经把该 logger 注入正式依赖图，但未调用 `fx.WithLogger`，因此 Fx 自身事件没有进入统一结构化日志链路。

本变更横跨 `common` 和 `user-service`：`common/runtime/logger` 提供无业务语义的 Fx event logger adapter，`common/runtime/observability/tracing` 保证 Fx 依赖图构造期即可提供可用 tracing provider，`user-service` composition root 负责选择在顶层 App 中启用该 adapter。该能力属于 `runtime-observability`，不进入 feature 包、`internal/shared` 或部署资产。

## Goals / Non-Goals

**Goals:**

- 让 user-service 正式 Fx App 使用同一个配置化 `*zap.Logger` 输出 Fx event。
- 保留共享 logger 的生命周期语义，由现有 `NewLogger` 在 Stop 阶段同步 logger。
- 使用 Fx 内置 `fxevent.ZapLogger`，消费当前 Fx 版本提供的 constructor、decorator、stub、Invoke、rollback、lifecycle 和 module trace 事件。
- 保持 event logger 快速、同步本地日志写入且非阻塞，不在 `LogEvent` 路径引入网络 I/O 或额外外部依赖。
- 让 `common/runtime/observability/tracing.NewFxProvider` 返回时已经持有非 nil `TracerProvider()`，满足 Redis、Gin、Ent 等 constructor 阶段的 tracing 依赖。

**Non-Goals:**

- 不新增日志配置项、日志采样策略或动态开关。
- 不改 HTTP API、OpenAPI 注解、数据库 schema、Atlas migration、RBAC policy 或部署清单。
- 不把 Fx event logger 放入 user-service feature、`internal/shared` 或 `internal/integration`。
- 不替换现有业务 logger helper、request ID、trace ID 或 access log 语义。

## Decisions

- 在 `common/runtime/logger` 中新增 `NewFxEventLogger(log *zap.Logger) fxevent.Logger`。
  理由：Fx event logger 是通用 runtime 观测 adapter，不包含 user-service 业务语义，适合与 logger primitive 同包维护。
  备选方案是在 `user-service/internal/bootstrap` 内定义 helper，但会让可复用的 Fx 日志适配逻辑被服务 composition root 私有化。

- `NewFxEventLogger` 使用 `log.Named("fx")` 构造 `fxevent.ZapLogger`。
  理由：命名 logger 能把 Fx 事件与业务日志区分，同时沿用同一 encoder、输出目标、字段和生命周期。
  备选方案是直接使用根 logger，但会降低日志过滤和定位 Fx 初始化问题的可读性。

- 使用 `UseLogLevel(zap.DebugLevel)` 和 `UseErrorLevel(zap.ErrorLevel)`。
  理由：常规构图与执行事件通常用于诊断，避免在 info 级别增加生产噪声；失败事件仍以 error 级别暴露。
  备选方案是使用 Fx 默认 level，但默认 console logger 行为和服务结构化日志策略不一致。

- 在 `AppOptions` 中追加 `fx.WithLogger(logger.NewFxEventLogger)`，并保持 `logger.NewLogger` 仍由 `fx.Provide` 注入。
  理由：`fx.WithLogger` 支持依赖注入式构造 event logger，可以复用当前 App 的 `*zap.Logger`，且顶层 composition root 是启用 App 级 Fx logger 的正确位置。
  备选方案是在每个模块内配置 logger，但 Fx logger 是 App 级行为，分散配置会产生重复和覆盖风险。

- `common/runtime/observability/tracing.NewFxProvider` 在 provider 构造阶段创建底层 tracing provider，并注册 no-op `OnStart` 与 `OnStop: provider.Shutdown`。
  理由：Redis、Gin、Ent 等 provider 在 constructor 阶段需要 `TracerProvider()`，延迟到 `OnStart` 初始化会导致构图失败；保留 no-op `OnStart` 可以确保后续 hook 启动失败时 Fx rollback 会执行 tracing shutdown。
  备选方案是让所有下游 provider 接受 nil tracing provider 并降级为 noop，但这会破坏正式依赖图的显式非 nil 契约，并把初始化时序问题扩散到多个 provider。

## Risks / Trade-offs

- [Risk] debug 级别 Fx event 可能在本地或 debug 环境产生更多日志。
  Mitigation: 常规事件使用 debug level，生产 info level 默认不会输出；失败事件保持 error level。

- [Risk] `fx.WithLogger` 构造依赖 `*zap.Logger`，logger provider 失败会影响 Fx event logger 初始化。
  Mitigation: logger provider 本来就是正式依赖图的一部分，失败应阻止 App 启动并由 Fx 构造错误暴露。

- [Risk] 测试中同时传入 `fx.NopLogger` 可能覆盖默认 AppOptions 的 Fx logger。
  Mitigation: 仅在需要静默 Fx 输出的测试里继续追加 `fx.NopLogger`；新增测试覆盖默认 `AppOptions` 包含自定义 Fx logger。

- [Risk] event logger 不能执行网络 I/O，否则会拖慢 Fx 初始化或关闭路径。
  Mitigation: 只使用 zap logger 的本地 `fxevent.ZapLogger` adapter，不在 `LogEvent` 中增加 hook、exporter 或远程调用。

- [Risk] tracing provider 在构造阶段初始化会让启用 OTLP 且配置无效时更早失败。
  Mitigation: 这是正式依赖图的期望行为；配置错误应阻止 App 构造或启动，并由 Fx 结构化日志暴露。

## Migration Plan

- 实施时先在 `common/runtime/logger` 增加 provider 和单元测试，再在 `user-service/internal/bootstrap/AppOptions` 接入 `fx.WithLogger`，随后调整 `common/runtime/observability/tracing` 的 Fx provider 初始化时序。
- 该变更无需数据迁移、API 发布协调或部署清单更新，可随普通 user-service 镜像发布。
- 回滚方式是移除 `fx.WithLogger(logger.NewFxEventLogger)` 和新增 provider；回滚后 Fx 事件恢复默认 logger，不影响业务请求处理。

## Open Questions

- 无待决问题。
