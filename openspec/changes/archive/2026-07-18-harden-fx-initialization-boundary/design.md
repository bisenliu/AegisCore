## Context

user-service 的正式应用由 `user-service/internal/bootstrap/AppOptions` 和 `AppModule` 组装。当前顶层未启用 `fx.RecoverFromPanics()`，同时 auth、permission、role 的部分 application constructor 使用 `panic` 表达可预期的缺失依赖。另一方面，`common/runtime/observability/tracing.NewFxProvider` 当前返回未启动 provider，并在 `OnStart` 才创建底层 tracer provider；但 user-service 的 Redis provider 在 constructor 阶段需要 tracing provider 进行 Redis instrumentation，存在初始化时序不一致。

本 change 横跨 `common/runtime/observability/tracing`、`common/runtime/datastore`、`user-service/internal/providers`、`user-service/internal/bootstrap` 和 auth/permission/role application constructor。它不改变 HTTP API、数据库 schema、OpenAPI 生成物、部署清单或 RBAC policy 数据模型。

## Goals / Non-Goals

**Goals:**

- 将 auth、permission、role 内部 constructor 中可预期的 nil 依赖错误改为返回 `error`，不保留旧 panic 语义。
- 让 Fx tracing provider 在 constructor 阶段提供非 nil、可用于 instrumentation 的 provider，并继续通过 lifecycle stop 关闭资源。
- 保持 Redis instrumentation 失败返回 error，并确保 user-service cache Redis provider 包装和传播该错误。
- 在 user-service composition root 启用 `fx.RecoverFromPanics()`，只作为 DI 初始化边界保护。
- 用测试覆盖 constructor error、Redis instrumentation error、Fx tracing provider 时序和顶层 panic recovery 行为。

**Non-Goals:**

- 不改变 Gin HTTP handler recovery、中间件链或 HTTP 错误响应契约。
- 不为 workerpool、scheduler、后台 goroutine 或 lifecycle hook 内运行期 panic 引入新的全局恢复策略。
- 不改变 PostgreSQL、Redis 配置格式、资源名、metrics label、span 属性或 OpenAPI 文档。
- 不保留 panic 行为的兼容分支、旧 constructor wrapper 或 deprecated API。

## Decisions

- constructor 可预期错误统一返回 `error`。
  备选方案是保留 panic 并依赖 `fx.RecoverFromPanics()` 兜底。最终不采用，因为缺失依赖属于可预期装配错误，应该通过 Fx 的标准 error path 暴露，panic recovery 只兜底未预期缺陷。

- tracing Fx provider 在构造阶段创建底层 provider。
  备选方案是在 Redis provider 中接受 nil tracer provider 并回退到全局 provider。最终不采用，因为这会让正式 graph 中的 tracing 依赖变成隐式降级，削弱 service runtime config 的单一来源。

- Redis instrumentation 继续保留 error 语义。
  备选方案是将 instrumentation panic 交给 Fx recovery。最终不采用，因为当前共享 datastore 已能返回 `instrument redis tracing` 错误，并且关闭 client；该语义更可测试也更符合资源 constructor 契约。

- `fx.RecoverFromPanics()` 放在 `AppOptions` 基础 options 中。
  备选方案是在各模块局部添加 recover option。最终不采用，因为 DI 初始化边界属于 user-service composition root 的统一策略，局部添加会造成保护范围不一致。

- 测试不新增生产专用测试开关。
  备选方案是为 tracing 或 Redis instrumentation 暴露新的生产注入点。最终不采用，失败注入应尽量使用现有 package-local fixture、Fx graph 测试或普通 constructor 断言。

## Risks / Trade-offs

- constructor 签名从单返回值变为 `(T, error)` 可能影响直接调用测试。缓解：同步更新 package-local 测试和 Fx provider 断言，保持正式 Fx graph 通过标准错误路径解析。
- tracing provider 提前创建可能把部分配置错误从 `App.Start` 提前到 `fx.New` 或 `app.Err()`。缓解：这是预期行为，配置和 exporter 构造错误应在启动前明确暴露。
- `fx.RecoverFromPanics()` 可能让某些初始化缺陷表现为 Fx error 而非进程 panic。缓解：测试断言错误包含 panic 内容，并在文档和规格中限定其只覆盖 DI 初始化边界。
- Redis provider 修正后可能暴露现有测试使用未启动 tracing provider 的问题。缓解：测试 fixture 应创建真实已初始化 provider，或通过 Fx graph 启动验证真实时序。

## Migration Plan

1. 更新 auth、permission、role constructor 签名并同步调用方和测试。
2. 更新 tracing Fx provider 的构造与 lifecycle 关闭时序，确保 constructor 阶段 `TracerProvider()` 非 nil。
3. 更新 cache Redis provider 和测试，覆盖 tracing provider、instrumentation error 和 Fx graph 实例化路径。
4. 在 `AppOptions` 加入 `fx.RecoverFromPanics()`，并增加顶层行为测试。
5. 运行相关包测试、`make user-service-architecture-lint`、`make lint` 和 `make verify`。

回滚策略：如出现启动期 tracing 或 Redis 初始化回归，回滚本 change 的代码提交和 OpenSpec change；无数据库、OpenAPI 或部署资产需要回滚。

## Open Questions

无。
