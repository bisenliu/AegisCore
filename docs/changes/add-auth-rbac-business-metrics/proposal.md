# Add auth RBAC business metrics

## What

为用户服务最关键的认证与 RBAC 风险路径补充低基数 Prometheus 业务指标，覆盖认证会话、token 校验、RBAC policy 同步和权限 route diff。

本变更在既有 `observability.metrics` provider、`/metrics` endpoint、HTTP server RED 指标和 runtime dependency 指标基础上增加 feature-owned business metrics：

- Auth 记录登录、refresh、logout current/all、token version mismatch 和 session purge submit failure。
- Permission/RBAC 记录 policy reload 编排结果、policy publish 结果、watcher version mismatch 和 route diff missing/stale 数量。
- 指标命名、label key 和 label value 继续遵循 `common/runtime/observability/metrics` 的低基数约定。
- 指标依赖通过 auth 和 permission feature Fx module 注入，不让 domain 层依赖 metrics。
- 指标记录失败不得影响认证、授权或 RBAC policy 同步主流程。

## Why

用户服务已经能暴露 runtime dependency metrics，包括 PostgreSQL、Redis、auth session purge workerpool、RBAC watcher 状态和 Casbin policy reload 状态。这些信号能说明运行时依赖是否健康，但还不能回答关键业务风险路径的结果分布：

- 登录失败是凭据错误、账号状态拒绝、系统异常还是 token/session 写入失败？
- Refresh 失败是输入/token/session 校验拒绝、token version mismatch、rotation session mismatch 还是系统异常？
- Logout current/all 是否成功，全部登出后的 session purge 提交失败是否发生？
- RBAC policy 在线变更后，本实例 reload 和 Redis version publish 哪一步失败？
- 其他副本是否检测到 policy version mismatch，需要补偿 reload？
- 当前注册路由与权限目录是否出现 missing/stale 漂移？

这些信息属于用户服务 feature 业务风险指标，不应该放进 `common/runtime/observability/metrics`。通过 feature-local recorder 和服务侧 Prometheus adapter，可以在保持行为不变的前提下，让部署侧持续观察认证与 RBAC 风险路径。

## Scope

包括：

- 在 auth feature application 或 transport 边界增加业务指标窄接口与 no-op 默认实现。
- 在 auth feature Fx module 中通过 `*commonmetrics.Provider` 提供 Prometheus recorder，并注入 auth command use case、token version validator 或认证 middleware adapter。
- 记录登录成功/失败，label 只使用固定 `operation=login`、`result=success|failure` 和有限 `reason`。
- 记录 refresh 成功/失败，区分 validation、token invalid/expired、session invalid/mismatch、token_version_mismatch、rotation/session persistence failure 和 system error 等固定原因。
- 记录 token version mismatch，覆盖 access token middleware 校验和 refresh session 校验中的 mismatch。
- 记录 logout current/all 成功/失败，label 使用固定 `operation=logout_current|logout_all`。
- 记录 auth session purge submit failure，作为全部登出后后台清理提交失败的业务风险信号；不替代现有 workerpool runtime 指标。
- 在 permission/RBAC application 或 infrastructure adapter 边界增加业务指标窄接口与 no-op 默认实现。
- 记录 policy local reload success/failure、policy publish success/failure、watcher remote reload success/failure 和 watcher version mismatch。
- 记录 route diff missing/stale 数量，使用 gauge 或每次查询更新的 latest gauge。
- 增加 feature application、transport 或 infrastructure 单元测试，验证关键记录点和 label 值。
- 更新 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 或 metrics README 中必要说明，明确 business metrics 归 feature/service 拥有，common metrics 包不承载用户服务业务语义。

不包括：

- 不记录用户 ID、用户名、角色 ID、权限 ID、session ID、token version、policy version、route template、raw path、IP、User-Agent 或错误消息全文作为 label。
- 不改变登录、refresh、logout、JWT 认证、授权、RBAC policy reload/publish/watcher 或 route diff 行为。
- 不改变 HTTP response、错误码、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- 不引入 tracing、dashboard、alert rules、ServiceMonitor、PodMonitor、Helm/Kubernetes 监控资产。
- 不把用户服务业务指标放入 `common/runtime/observability/metrics`；common 只保留无业务语义 provider、label 约定和 runtime collector。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `observability.metrics.enabled: true` 时，`GET /metrics` 能看到 auth 和 RBAC business metrics。
- 登录、refresh、logout current/all、token version mismatch、policy reload、policy publish、watcher version mismatch 和 route diff 关键路径均有可 scrape 指标。
- 预期业务拒绝和系统异常在指标上可通过固定 `reason` 或 `result` 区分。
- 所有业务指标 label 均为低基数固定枚举，不包含用户、角色、权限、session、token、policy version、route template 或错误详情。
- Metrics disabled 或 recorder 注册失败以外的单次记录失败不影响业务流程；disabled 模式使用 no-op recorder。
- Auth 与 permission domain 层不导入 metrics、Prometheus、Gin、Ent、Redis 或 Fx。
- 增加针对 feature application、transport 或 infrastructure 的测试，覆盖成功/失败记录点和 token version mismatch、route diff 数量等关键信号。
- 实现后 `make test-user-service` 和 `make architecture-lint` 通过，或明确说明外部依赖导致未能运行。
