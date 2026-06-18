# Tasks

## 1. Preparation

- [x] 1.1 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、本 change 的 `proposal.md` 和 `design.md`。
- [x] 1.2 确认本 change 使用 `docs/changes/add-auth-rbac-business-metrics/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 1.3 梳理 auth login、refresh、logout current/all、token version validator 和 Redis session purge submit failure 的当前代码路径。
- [x] 1.4 梳理 permission policy refresh coordinator、Redis watcher、Casbin reload runtime metrics 和 route diff query 的当前代码路径。
- [x] 1.5 确认本变更不改变 HTTP response、错误码、认证授权行为、RBAC policy 同步行为、schema、migration 或 Redis key。

## 2. Auth Metrics Contract

- [x] 2.1 在 auth feature application 层新增业务 metrics 窄接口、固定 operation/result/reason/source 常量和 no-op 实现。
- [x] 2.2 reason/source 常量只包含固定枚举，不接收用户输入或错误字符串。
- [x] 2.3 在 `UseCaseDepsParams` 增加 optional auth metrics dependency，nil 时使用 no-op。
- [x] 2.4 确认 auth domain 层不导入 metrics、Prometheus、Fx、Gin、Ent、Redis 或 SQL。

## 3. Auth Prometheus Recorder

- [x] 3.1 在 auth feature wiring 或 feature-local metrics adapter 中实现 Prometheus recorder。
- [x] 3.2 指标 `aegiscore_user_service_auth_operations_total` 使用 counter，label 为 `operation`,`result`,`reason`。
- [x] 3.3 指标 `aegiscore_user_service_auth_token_version_mismatches_total` 使用 counter，label 为 `source`。
- [x] 3.4 指标 `aegiscore_user_service_auth_session_purge_submit_failures_total` 使用 counter，无业务实体 label。
- [x] 3.5 Metrics disabled 时返回 no-op，不注册 collector。
- [x] 3.6 注册失败时让 Fx 启动失败；单次记录不返回错误，不影响业务流程。

## 4. Login Metrics

- [x] 4.1 在登录输入校验失败时记录 `login/failure/validation_failed`。
- [x] 4.2 在凭据错误、用户状态拒绝和系统异常时分别记录固定 failure reason。
- [x] 4.3 必须改密用户成功签发受限 token 时记录 login success；签发失败记录固定 failure reason。
- [x] 4.4 普通登录 token/session 全部成功后记录 login success。
- [x] 4.5 token 签发失败和 session 创建失败可区分时分别记录 `token_issue_failed`、`session_create_failed`；否则保守记录 `system_error` 并在设计注释中说明。
- [x] 4.6 更新 auth command tests，覆盖 login success 和至少两个 login failure reason。

## 5. Refresh Metrics

- [x] 5.1 在 refresh 输入校验失败时记录 `refresh/failure/validation_failed`。
- [x] 5.2 refresh token parse 失败按固定枚举记录 invalid 或 expired。
- [x] 5.3 refresh session 校验失败按固定枚举记录 invalid、mismatch、token version mismatch 或 system error。
- [x] 5.4 refresh token version mismatch 同时记录 `auth_token_version_mismatches_total{source="refresh_token"}`。
- [x] 5.5 非 rotation refresh 成功记录 success，失败按 token issue 或 session/system reason 记录。
- [x] 5.6 rotation refresh token 签发失败记录 `token_issue_failed`，session rotate 失败记录 `session_rotate_failed`。
- [x] 5.7 更新 refresh use case tests，覆盖 success、token version mismatch 和一个 system failure reason。

## 6. Access Token Version Mismatch Metrics

- [x] 6.1 在用户服务侧增加 token version validator wrapper，包装现有 `commonauth.TokenVersionValidator`。
- [x] 6.2 wrapper 只在 `errors.Is(err, commonauth.ErrTokenVersionMismatch)` 时记录 `source=access_token`。
- [x] 6.3 wrapper 原样返回 validator 的 error，不改变 middleware response 语义。
- [x] 6.4 通过 auth feature Fx 或 providers wiring 注入 wrapper，避免修改 common auth middleware 的业务指标职责。
- [x] 6.5 增加 wrapper 单元测试，覆盖 mismatch 记录、非 mismatch 不记录和 nil/no-op 场景。

## 7. Logout And Session Purge Metrics

- [x] 7.1 `LogoutCurrentSession` auth context 缺失记录 `logout_current/failure/auth_context_missing`。
- [x] 7.2 `LogoutCurrentSession` session delete 失败记录 `logout_current/failure/session_delete_failed`。
- [x] 7.3 `LogoutCurrentSession` 成功记录 `logout_current/success/none`。
- [x] 7.4 `LogoutAllSessions` auth context 缺失记录 `logout_all/failure/auth_context_missing`。
- [x] 7.5 `LogoutAllSessions` revoke 失败记录 `logout_all/failure/session_revoke_failed`。
- [x] 7.6 `LogoutAllSessions` 成功记录 `logout_all/success/none`。
- [x] 7.7 在 auth session purge submit failure 的实际发生点记录 `auth_session_purge_submit_failures_total`。
- [x] 7.8 确认 session purge metrics 不记录 user ID、purge key、session key 或错误文本。
- [x] 7.9 更新 logout 和 Redis session purge tests。

## 8. RBAC Metrics Contract

- [x] 8.1 在 permission application 层新增 RBAC business metrics 窄接口、固定 operation/result/reason/source/kind 常量和 no-op 实现。
- [x] 8.2 在 permission feature Fx 中提供 Prometheus recorder，并在 metrics disabled 时返回 no-op。
- [x] 8.3 指标 `aegiscore_user_service_rbac_policy_sync_operations_total` 使用 counter，label 为 `operation`,`result`,`reason`,`source`。
- [x] 8.4 指标 `aegiscore_user_service_rbac_policy_version_mismatches_total` 使用 counter，label 为 `source`。
- [x] 8.5 指标 `aegiscore_user_service_permission_route_diff` 使用 gauge，label 为 `kind=missing|stale`。
- [x] 8.6 确认 permission domain 层不导入 metrics、Prometheus、Fx、Gin、Ent、Redis 或 SQL。

## 9. Policy Refresh Metrics

- [x] 9.1 `PolicyRefreshCoordinator.NotifyPolicyChanged` 本实例 reload 成功记录 `policy_reload/success/none/local_change`。
- [x] 9.2 本实例 reload 失败记录 `policy_reload/failure/reload_failed/local_change`，并保持不 publish 的现有行为。
- [x] 9.3 Redis policy version publish 成功记录 `policy_publish/success/none/local_change`。
- [x] 9.4 Redis policy version publish 失败记录 `policy_publish/failure/publish_failed/local_change`。
- [x] 9.5 更新 `policy_sync_test.go`，覆盖 reload success/publish success、reload failure 和 publish failure 的 metrics 调用。

## 10. Watcher Metrics

- [x] 10.1 `Watcher.CheckVersion` 读取 remote version 失败时按固定 reason 记录 watcher check failure，或在代码注释中明确仅保留日志。
- [x] 10.2 remote version 大于 local version 时记录 `rbac_policy_version_mismatches_total{source="watcher_version_check"}`。
- [x] 10.3 Pub/Sub payload 触发 remote reload 时记录 `watcher_reload` success/failure，source 为 `watcher_pubsub`。
- [x] 10.4 定时版本补偿触发 remote reload 时记录 `watcher_reload` success/failure，source 为 `watcher_version_check`。
- [x] 10.5 不把 policy version、instance ID、reason payload 或错误消息写入 metric label。
- [x] 10.6 更新 watcher tests，覆盖 version mismatch、remote reload success/failure 和 label source。

## 11. Route Diff Metrics

- [x] 11.1 `GetRouteDiff` 成功计算后记录 missing/stale 数量。
- [x] 11.2 scanner/store 失败时保持原错误返回，不更新 route diff gauge。
- [x] 11.3 route diff metric 只使用 `kind=missing|stale`，不记录 method、route template、permission code 或 permission ID。
- [x] 11.4 更新 route diff tests，覆盖 missing/stale gauge 更新和错误场景。

## 12. Documentation

- [x] 12.1 更新 `docs/ARCHITECTURE.md`，说明用户服务 feature-owned business metrics 的归属和 label 边界。
- [x] 12.2 更新 `docs/DEVELOPMENT.md` 或 `common/runtime/observability/metrics/README.md`，说明 business metrics 不属于 common runtime metrics。
- [x] 12.3 文档明确禁止在 business metric label 中放用户 ID、角色 ID、权限 ID、session ID、token version、policy version、route template 或错误消息。
- [x] 12.4 确认文档不新增 dashboard、alert rules、tracing、ServiceMonitor、PodMonitor、Helm/Kubernetes artifact。
- [x] 12.5 确认文档没有重新引入 OpenSpec/OPSX 流程或目录。

## 13. Tests And Verification

- [x] 13.1 格式化修改过的 Go 文件：

```bash
gofmt -w <changed-go-files>
```

- [x] 13.2 运行 auth feature 相关测试：

```bash
cd user-service && go test ./internal/features/auth/...
```

- [x] 13.3 运行 permission feature 相关测试：

```bash
cd user-service && go test ./internal/features/permission/...
```

- [x] 13.4 运行 provider/router 相关测试，如 token validator wrapper 或 Fx wiring 影响 providers：

```bash
cd user-service && go test ./internal/providers/...
```

- [x] 13.5 运行用户服务测试：

```bash
make test-user-service
```

- [x] 13.6 运行架构边界检查：

```bash
make architecture-lint
```

- [x] 13.7 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 13.8 在 metrics enabled 的测试环境中确认 `/metrics` 包含 auth/RBAC business metrics，且 label 值均为固定枚举。

## 14. Guardrails

- [x] 14.1 不改变认证、授权、RBAC policy 同步或 route diff 行为。
- [x] 14.2 不修改 HTTP API response、错误码、数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- [x] 14.3 不新增 tracing、dashboard、alert rules 或部署监控资源。
- [x] 14.4 不把用户服务业务指标定义放进 `common/runtime/observability/metrics`。
- [x] 14.5 不在 label 中放高基数或敏感值。
- [x] 14.6 不新增 `openspec/` 或 `docs/opsx/`。
