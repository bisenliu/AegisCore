# Design

## Overview

本变更为用户服务增加 feature-owned business metrics。实现上分三层：

```text
auth feature
  application/command + validators or transport adapter
  -> AuthMetrics narrow interface
  -> Prometheus recorder provided by auth fx

permission feature
  application policy refresh + watcher + route diff query
  -> RBACMetrics narrow interface
  -> Prometheus recorder provided by permission fx

common/runtime/observability/metrics
  -> existing Provider, labels, Register/Gatherer only
  -> no user-service business metric definitions
```

核心原则：

- feature application 只依赖窄 recorder interface，不依赖 Prometheus collector 类型。
- domain 层不感知 metrics。
- common metrics package 不新增 auth/RBAC 业务指标定义。
- recorder 默认 no-op，metrics disabled 时零副作用。
- 指标记录失败不得改变业务返回值或副作用顺序。
- label 只允许固定枚举，禁止业务实体 ID 和动态错误详情。

## Current State

已有基础：

- `common/runtime/observability/metrics.Provider` 使用独立 Prometheus registry，disabled 模式零副作用。
- 用户服务已在 metrics enabled 时暴露配置化 `/metrics` endpoint。
- HTTP server RED 指标、runtime dependency 指标、RBAC watcher 状态和 Casbin policy reload runtime 指标已存在。
- Auth command use case 通过 `UseCaseDeps` 聚合 credentials、tokens、sessions 和 config。
- Auth token version validator 通过 `commonauth.TokenVersionValidator` 接入认证 middleware。
- Auth session purge workerpool 已有 runtime 指标，但业务上仍需要记录 submit failure 这类全部登出风险事件。
- Permission `PolicyRefreshCoordinator` 编排本实例 reload 和 Redis policy version publish。
- Permission Redis watcher 负责 Pub/Sub 和定时版本补偿，检测 remote version 大于 local version 时 reload。
- Permission route diff query 返回 missing/stale 差异，但当前没有指标输出数量。

约束：

- 日志消息如新增必须是英文，代码注释使用中文。
- 不改变现有业务行为、response、错误码、schema 或 Redis key。
- 不把 business metric recorder 放入 `common/runtime/observability/metrics`。
- 不新增 OpenSpec/OPSX 工件。

## Metric Naming And Labels

业务指标建议使用 `aegiscore_user_service_` 前缀，避免与 common runtime 指标混淆。

通用 label：

| Label | Allowed values | Notes |
|---|---|---|
| `operation` | `login`, `refresh`, `logout_current`, `logout_all`, `policy_reload`, `policy_publish`, `watcher_reload`, `route_diff` | 固定枚举，不来自请求输入 |
| `result` | `success`, `failure` | 固定枚举 |
| `reason` | 见下方枚举 | 固定枚举，不使用错误字符串 |
| `source` | `access_token`, `refresh_token`, `local_change`, `watcher_pubsub`, `watcher_version_check`, `route_diff_query` | 固定枚举 |
| `kind` | `missing`, `stale` | route diff 固定枚举 |

禁止 label value：

- user ID、username、role ID、permission ID、session ID、token version、policy version。
- route template、raw path、URL query、IP、User-Agent、trace/span/request ID。
- Redis key、SQL、JWT、Authorization header、Cookie、错误消息全文。

## Auth Metrics Interface

建议在 auth application 层新增窄接口，例如 `application/metrics.go`：

```go
type Metrics interface {
    RecordAuthOperation(ctx context.Context, operation string, result string, reason string)
    RecordTokenVersionMismatch(ctx context.Context, source string)
    RecordSessionPurgeSubmitFailure(ctx context.Context)
}
```

也可以按可读性拆成更具体方法：

```go
type Metrics interface {
    LoginSucceeded(context.Context)
    LoginFailed(context.Context, reason string)
    RefreshSucceeded(context.Context)
    RefreshFailed(context.Context, reason string)
    LogoutSucceeded(context.Context, operation string)
    LogoutFailed(context.Context, operation string, reason string)
    TokenVersionMismatch(context.Context, source string)
    SessionPurgeSubmitFailed(context.Context)
}
```

推荐第二种，因为调用点可读性更好，且 reason/source 可在 recorder 内校验或归一化为固定枚举。

`UseCaseDepsParams` 增加 optional metrics：

```go
Metrics authapplication.Metrics `optional:"true"`
```

`NewUseCaseDeps` 中 nil 时使用 no-op。`UseCaseDeps` 保存 metrics，command use case 通过 `defer` 或显式分支记录结果。

### Auth Metric Names

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `aegiscore_user_service_auth_operations_total` | counter | `operation`,`result`,`reason` | login/refresh/logout 业务结果 |
| `aegiscore_user_service_auth_token_version_mismatches_total` | counter | `source` | access/refresh token version mismatch |
| `aegiscore_user_service_auth_session_purge_submit_failures_total` | counter | none | logout all 后 session purge 提交失败 |

`auth_operations_total` 的成功 reason 固定为 `none`。

### Auth Reasons

登录 failure reason：

- `validation_failed`
- `credential_invalid`
- `user_status_rejected`
- `password_change_required_issue_failed`
- `token_issue_failed`
- `session_create_failed`
- `system_error`

Refresh failure reason：

- `validation_failed`
- `refresh_token_invalid`
- `refresh_token_expired`
- `refresh_session_invalid`
- `refresh_session_mismatch`
- `token_version_mismatch`
- `token_issue_failed`
- `session_rotate_failed`
- `session_create_failed`
- `system_error`

Logout failure reason：

- `auth_context_missing`
- `session_delete_failed`
- `session_revoke_failed`
- `system_error`

这些枚举应集中为 const，测试固定 label value。错误归类可以使用 `errors.Is` / `errors.As` 映射已有 domain/common errors；无法稳定归类的错误用 `system_error`。

## Auth Recording Points

### Login

`loginUseCase.Login`：

1. 输入校验失败记录 `operation=login,result=failure,reason=validation_failed`。
2. 凭据校验失败按已有错误归类为 `credential_invalid`、`user_status_rejected` 或 `system_error`。
3. 必须改密用户成功签发受限 token 时记录 success；签发失败记录 `password_change_required_issue_failed`。
4. 普通 token pair 签发或 session 创建成功后记录 success；失败按 `token_issue_failed` 或 `session_create_failed` 记录。

若 `issueTokenPair` 当前无法区分 token issue 和 session create 失败，可将内部步骤展开为可记录的私有 helper，保持行为不变。

### Refresh

`refreshTokenUseCase.Refresh`：

1. 输入校验失败记录 `validation_failed`。
2. `ParseRefreshToken` 失败按 JWT/common auth error 归为 `refresh_token_invalid` 或 `refresh_token_expired`。
3. `ValidateRefreshSession` 失败按 session/domain/common auth error 归为 `refresh_session_invalid`、`refresh_session_mismatch`、`token_version_mismatch` 或 `system_error`。
4. token version mismatch 同时记录 `auth_token_version_mismatches_total{source="refresh_token"}`。
5. 非 rotation 成功记录 refresh success；issue token 失败记录 `token_issue_failed`。
6. rotation 模式中 token 签发失败记录 `token_issue_failed`，session rotate 失败记录 `session_rotate_failed`。

### Access Token Version Mismatch

当前 access token version mismatch 发生在 common auth middleware 调用 `TokenVersionValidator` 后。为了避免 common middleware 承载用户服务业务指标，推荐在用户服务侧包一层 validator：

```go
type metricsTokenVersionValidator struct {
    next    commonauth.TokenVersionValidator
    metrics authapplication.Metrics
}
```

该 wrapper 位于 user-service auth application/validators 或 providers 层，注入到 `providers.routes.go` 的 `TokenVersions` 依赖链。它只在 `errors.Is(err, commonauth.ErrTokenVersionMismatch)` 时记录 `source=access_token`，再原样返回错误。

### Logout

`logoutCurrentSessionUseCase.LogoutCurrentSession`：

- auth context 缺失记录 `logout_current/failure/auth_context_missing`。
- session delete 失败记录 `logout_current/failure/session_delete_failed`。
- 成功记录 `logout_current/success/none`。

`logoutAllSessionsUseCase.LogoutAllSessions`：

- auth context 缺失记录 `logout_all/failure/auth_context_missing`。
- `RevokeAllUserSessions` 返回错误记录 `logout_all/failure/session_revoke_failed`。
- 成功记录 `logout_all/success/none`。

### Session Purge Submit Failure

当前后台 purge submit failure 最接近业务风险的产生点在 auth Redis session adapter 的 `DeleteAllUserSessions` / `RevokeAllUserSessions` 路径，而 workerpool runtime 指标只能说明 pool submit rejected。设计要求：

- auth Redis adapter 增加 optional narrow metrics dependency，或通过 application session lifecycle 包装记录。
- 只有在全部登出 detach 成功后提交 purge task 失败时记录 `auth_session_purge_submit_failures_total`。
- 不记录 user ID、purge key、session key 或错误文本到 metric label。
- 保留现有日志，可继续记录必要调试字段，但不能影响指标 label。

## RBAC Metrics Interface

建议在 permission application 层新增窄接口，例如：

```go
type Metrics interface {
    PolicyReloadSucceeded(ctx context.Context, source string)
    PolicyReloadFailed(ctx context.Context, source string, reason string)
    PolicyPublishSucceeded(ctx context.Context)
    PolicyPublishFailed(ctx context.Context, reason string)
    WatcherVersionMismatch(ctx context.Context)
    RouteDiffObserved(ctx context.Context, missing int, stale int)
}
```

Nil 使用 no-op。Prometheus 实现由 permission `fx.go` 提供，依赖 `*commonmetrics.Provider`。

### RBAC Metric Names

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `aegiscore_user_service_rbac_policy_sync_operations_total` | counter | `operation`,`result`,`reason`,`source` | reload/publish/watcher reload 结果 |
| `aegiscore_user_service_rbac_policy_version_mismatches_total` | counter | `source` | watcher 检测远端版本领先本地 |
| `aegiscore_user_service_permission_route_diff` | gauge | `kind` | 最近一次 route diff 查询的 missing/stale 数量 |

`source` 固定：

- `local_change`：HTTP RBAC 管理写路径触发的本实例 reload。
- `watcher_pubsub`：Pub/Sub 消息触发的 remote reload。
- `watcher_version_check`：定时版本补偿触发的 remote reload 或 mismatch。
- `route_diff_query`：route diff 查询。

`reason` 固定：

- `none`
- `reload_failed`
- `publish_failed`
- `store_unavailable`
- `message_invalid`
- `system_error`

## RBAC Recording Points

### PolicyRefreshCoordinator

`PolicyRefreshCoordinator.NotifyPolicyChanged`：

1. 本实例 `engine.Reload` 成功记录 `operation=policy_reload,result=success,source=local_change,reason=none`。
2. 本实例 reload 失败记录 `policy_reload/failure/reload_failed/local_change` 并保持现有 return 行为。
3. `publisher.PublishPolicyChanged` 成功记录 `operation=policy_publish,result=success,source=local_change,reason=none`。
4. publish 失败记录 `policy_publish/failure/publish_failed/local_change` 并保持现有 return 行为。

已有 Casbin reload runtime metrics 可以继续存在；本变更的 policy sync business metrics 用于区分在线业务变更编排中的 reload/publish 结果。

### Watcher

`Watcher.CheckVersion`：

- 读取 remote version 失败不记录 version mismatch，可记录 `policy_sync_operations_total{operation="watcher_version_check",result="failure",reason="store_unavailable",source="watcher_version_check"}`。
- remote version 大于 local version 时记录 `rbac_policy_version_mismatches_total{source="watcher_version_check"}`。
- 不把 local/remote version 放入 label。

`Watcher.HandlePayload` / `reloadIfNewer`：

- 消息解析失败可记录 `operation=watcher_message,result=failure,reason=message_invalid,source=watcher_pubsub`，或只保留日志；若记录，operation 必须纳入固定枚举。
- remote reload 成功/失败按 `source=watcher_pubsub` 或 `source=watcher_version_check` 记录。
- 为区分来源，可将 `reloadIfNewer` 的 reason/source 参数改为固定 source enum，或增加内部参数，不改变外部行为。

### Route Diff

`permissionQueryService.GetRouteDiff`：

- 成功计算 missing/stale 后调用 `RouteDiffObserved(ctx, len(missing), len(stale))`。
- 推荐 gauge：
  - `aegiscore_user_service_permission_route_diff{kind="missing"}`
  - `aegiscore_user_service_permission_route_diff{kind="stale"}`
- scanner/store 失败时不更新 gauge，避免把旧成功结果清零造成误导；错误仍由调用方返回。
- 不以 route template、method、permission code 作为 label。

## Prometheus Recorder Placement

业务 Prometheus recorder 建议放在各 feature 内部，例如：

- `user-service/internal/features/auth/application/metrics.go`：interface、const、no-op。
- `user-service/internal/features/auth/infrastructure/metrics/prometheus.go` 或 `auth/fx.go` 同包小 provider：Prometheus 实现。
- `user-service/internal/features/permission/application/metrics.go`：interface、const、no-op。
- `user-service/internal/features/permission/infrastructure/metrics/prometheus.go` 或 `permission/fx.go` 同包小 provider：Prometheus 实现。

如果为了避免新增 `infrastructure/metrics` 目录，也可以把 recorder provider 放在 feature 根 `fx.go` 同包文件中；它属于 feature wiring，不是 domain 或 common。无论放在哪里，都不得让 domain 导入 metrics。

Prometheus collector 注册策略：

- 构造 recorder 时检查 provider enabled；disabled 返回 no-op。
- counter/gauge 通过 provider registerer 创建或注册。
- duplicate registration 复用 `Provider.Register` 已有策略，Fx 启动时注册错误应返回 error。
- 单次 `Inc` / `Set` 不应返回 error；业务流程不感知指标失败。

## Tests

Auth tests：

- Login success 记录 success。
- Login validation/credential/status/system failure 至少覆盖两个固定 failure reason。
- Refresh success 记录 success。
- Refresh token version mismatch 记录 refresh failure 和 `source=refresh_token` mismatch。
- Logout current/all success/failure 记录对应 operation。
- Access token validator wrapper 在 mismatch 时记录 `source=access_token` 并原样返回错误。
- Session purge submit failure 记录一次，且不影响原有错误返回语义。

Permission tests：

- `PolicyRefreshCoordinator` reload success + publish success 记录两个 success。
- Reload failure 记录 reload failure 且不 publish。
- Publish failure 记录 publish failure。
- Watcher version mismatch 记录 mismatch counter，remote reload success/failure 记录固定 source。
- Route diff success 更新 missing/stale gauge；scanner/store 失败不更新或测试保持旧值。

Provider/metrics tests：

- Metrics disabled 时 feature recorder 为 no-op。
- Gathered metric labels 只包含固定枚举，不含 user/session/role/permission/token/policy version。
- Existing runtime dependency metrics tests 继续通过，不需要修改 common metrics package 以承载业务指标。

## Risks / Trade-offs

- 业务指标在 application 层记录会靠近业务结果，但 access token mismatch 发生在 middleware 链，需要用户服务侧 validator wrapper。这样避免污染 common middleware。
- Casbin reload runtime metrics 已存在；新增 policy sync business metrics 可能看起来重复。两者口径不同：runtime 指标看 engine reload，business 指标看在线 RBAC 变更编排和跨副本同步结果。
- Route diff gauge 只有查询发生时更新，不代表后台持续扫描状态。当前 route diff 是只读诊断接口，本变更不新增定时扫描任务。
- 错误原因归类需要保守，无法稳定识别时统一为 `system_error`，避免把错误文本写入 label。
- Session purge submit failure 最准确的点在 Redis infrastructure adapter；为保持分层，只注入窄 metrics interface，不让 adapter 依赖 Prometheus。
