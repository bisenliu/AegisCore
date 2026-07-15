## Context

`common/runtime/logger` 当前通过 `New`、`NewWithConfig` 和 Fx provider `NewLogger` 构造 `*zap.Logger`，内部 `newLogger` 会调用 `SetDefault` 覆盖进程级默认 logger。该默认 logger 同时被 `FromContext`、`WithContext(nil, nil)`、`NamedComponent(nil, ...)` 和包级 `Info/Warn/Error/Debug` 作为兜底使用。

user-service 的正式 App 已经通过 Fx 装配出服务级 `*zap.Logger`，并由 HTTP middleware、provider、feature composition root 和部分 application/infrastructure 使用。问题在于构造 logger 本身也会写入全局默认状态，部分 feature application 或关键 infrastructure 在正式主路径仍可通过 package-level 默认 logger 获得日志实例，导致多个测试 App、CLI 或局部 logger 构造在同一进程内互相覆盖。

本变更横跨 `common` 与 `user-service`，只改变 Go 代码和 OpenSpec/架构检查，不影响 HTTP API、数据库 migration、OpenAPI 生成物、部署清单、Prometheus/Grafana 资产或 request ID/trace ID 外部传播契约。日志输出契约保持当前稳定字段、message、level、logger name 和敏感信息过滤规则。

## Goals / Non-Goals

**Goals:**

- 让 `common/runtime/logger.New`、`NewWithConfig` 和 `NewLogger` 只构造并返回 logger，不再隐式调用 `SetDefault`。
- 保留 `NewLogger` 在 Fx `OnStop` 中对正式 logger 执行 `Sync` 的既有关闭责任。
- 将 user、auth、role、permission application 与关键 infrastructure 的日志依赖改为 constructor 参数或明确 request context logger 路径。
- 保留 common 层确有必要的无注入兜底 API，但正式 user-service App 不安装、不恢复也不持有默认 logger。
- 增加架构检查或静态约束，阻止 feature application 在正式主路径重新依赖 package-level 默认 logger。
- 保持日志字段、日志级别、输出编码、request ID、trace ID 和 span ID 关联语义不变。

**Non-Goals:**

- 不新增默认 logger 的 Fx lifecycle owner、互斥锁协议、安装/恢复栈或多 App 协调机制。
- 不更换 Zap，不引入统一领域 logger interface，也不为每个日志调用创建领域 port。
- 不修改 HTTP API、OpenAPI、Ent schema、Atlas migration、Redis/PostgreSQL 数据结构或 RBAC policy 契约。
- 不改变 request ID、trace ID、span ID 的外部传播契约。
- 不通过生产兼容分支保留构造函数覆盖默认 logger 的旧行为。

## Decisions

### Decision 1: logger 构造函数不再写进程全局默认状态

`common/runtime/logger.newLogger` 将只负责创建配置化 `*zap.Logger` 并附加 service/env 字段。`New`、`NewWithConfig` 和 `NewLogger` 不再调用 `SetDefault`，调用方若确实需要修改默认 logger，必须显式调用 `SetDefault`。

备选方案是让 `NewLogger` 安装默认 logger 并在 `OnStop` 恢复旧值。该方案会引入多 App 并行时的恢复顺序协议，并让构造行为继续依赖可变进程状态，不满足测试隔离目标，因此不采用。

### Decision 2: 正式 App 使用 Fx 提供的 logger 实例作为依赖源

user-service provider 和 feature composition root 继续从 Fx 注入服务级 `*zap.Logger`，然后显式传入 user、auth、role、permission application constructor 或 infrastructure adapter constructor。需要组件命名时使用 `logger.NamedComponent(base, name, component)` 或 `base.Named(...)` 从注入实例派生。

备选方案是保留 application 内部调用 `logger.FromContext(context.Background())` 或 `logger.NamedComponent(nil, ...)`。该方案仍依赖默认 logger fallback，正式主路径不可追踪，因此只允许留在 common fallback 或非正式主路径场景。

### Decision 3: request-scoped 日志继续通过 context logger 传播

HTTP middleware、binding、controller 或 request lifecycle 中已有 context logger 路径继续使用 `logger.ToContext`、`logger.WithContext`、`logger.FromContext` 和 `logger.WithRequestID`。当 request context 中已明确写入 logger 时，应用日志仍自动带 `request_id`、`trace_id`、`span_id`。

备选方案是完全移除 `FromContext` 的默认 fallback。该方案会破坏 common helper 的无注入兜底 API，并扩大迁移范围，不符合保留 common 层必要兜底能力的目标，因此不采用。

### Decision 4: 架构检查聚焦正式 feature 主路径

新增或扩展 `make user-service-architecture-lint` 覆盖的静态检查，禁止 `user-service/internal/features/{user,auth,role,permission}/application` 和关键 infrastructure 在生产文件中调用 `logger.SetDefault`、`logger.FromContext(context.Background())`、包级 `logger.Info/Warn/Error/Debug` 或 `logger.NamedComponent(nil, ...)` 作为正式主路径 logger 来源。测试文件可使用局部 logger 或显式 context logger；必须覆盖默认 fallback 的测试应保存并恢复进程默认状态。

备选方案是只依赖 code review。该方案无法防止后续回归，不满足验收中的静态约束要求，因此不采用。

### Decision 5: 不改变部署和观测资产

本变更只改变 logger 依赖来源，不改变日志字段、level、格式、stdout/stderr 分流、metrics、tracing、dashboard 或 alert。`deployments/` 不应产生变更，OpenAPI 生成物和数据库 migration 也不应产生变更。

备选方案是同步调整日志输出格式或字段命名。该方案会扩大观测契约影响，并非本问题必要条件，因此不采用。

## Risks / Trade-offs

- [Risk] 某些当前隐式依赖默认 logger 的路径在迁移中遗漏，导致运行时退化为 nop fallback 或测试无法捕获预期日志。→ Mitigation: 通过 Grep/架构 lint 搜索 feature application/infrastructure 中的 package-level logger 入口，并为关键路径更新测试 fixture。
- [Risk] 移除构造副作用后，依赖 `NewWithConfig` 自动影响 `logger.FromContext` 的旧测试会失败。→ Mitigation: 将这些测试改为显式 `logger.ToContext`、显式注入 logger，或在专门测试默认 fallback 时显式 `SetDefault` 并 cleanup 恢复。
- [Risk] constructor 增加 `*zap.Logger` 可能扩大 application 参数列表。→ Mitigation: 只在实际记录日志的 service/use case/adapter 中注入 logger；不为没有日志行为的组件添加无用依赖，不新增泛化领域 logger port。
- [Risk] App Stop 的 logger `Sync` 责任被误删。→ Mitigation: 保留 `common/runtime/logger.NewLogger` 的 Fx `OnStop` hook，并增加或更新测试验证 `Sync` 仍由 lifecycle 执行。
- [Risk] 静态约束过宽误伤 common fallback 或测试场景。→ Mitigation: 检查范围限定为 user-service feature production 路径和关键 infrastructure；需要覆盖默认 fallback 的测试使用明确例外模式。

## Migration Plan

1. 在 `common/runtime/logger` 移除 `newLogger` 对 `SetDefault` 的调用，更新注释和单元测试，验证单独构造函数不改变默认 logger。
2. 审查 user、auth、role、permission application 与关键 infrastructure 的 logger 使用点，将正式主路径改为 constructor 注入或 request context logger。
3. 更新 `user-service/internal/providers/` 和 feature composition provider，将服务级 `*zap.Logger` 显式传入需要日志的 use case、service、store、policy sync、watcher 或 adapter。
4. 更新测试 fixture 和 mock 构造，确保并行构造多个测试 App 或多个 logger 实例时不互相覆盖默认 logger。
5. 增加或扩展架构 lint，阻止 feature application 重新依赖 package-level 默认 logger。
6. 执行 `cd common && go test ./runtime/logger ./http/... -count=1`、四个 feature 的 application/infrastructure 测试、`make user-service-architecture-lint` 和 `openspec validate remove-global-logger-dependencies`。
7. 暂存本次预期代码、规格和文档变更后执行 `make lint` 与 `make verify`。

Rollback strategy: 如果实施后出现不可接受的问题，回滚本 change 的代码和规格变更即可恢复旧构造副作用。不得以新增默认 logger lifecycle owner 或恢复栈作为回滚补丁，因为这会引入本变更明确排除的进程级协调协议。

## Open Questions

- 无待决问题。实施时若发现新的正式主路径依赖默认 logger，应优先按显式注入迁移；只有 common 层业务中立 fallback 需要保留默认 logger。
