## Why

当前 `common/security/casbin` 和 `common/http/middleware` 存在导出 helper 形态偏重复的问题：package-level `Authorize` 与 `Enforce` 的差异仅是将拒绝结果转换为 `ErrDenied`，`Auth()` 也只是无 token version 校验的简写且仓库内没有调用方。它们会扩大共享模块的公共 API 面，使后续服务复用时难以区分推荐入口和兼容入口。

本变更聚焦前期确认真实存在或部分存在的第 1 项和第 3 项；第 2 项 `redisScore`/`redisScoreFloat` 分别服务字符串参数和 `float64` score，不纳入本次调整。

## What Changes

- 收紧 `common/security/casbin` 的授权 helper 形态，保留返回原始授权结果的 `Enforce`，并让 `Authorizer.Authorize` 直接表达“拒绝即错误”的语义。
- 评估并移除或废弃未被仓库内使用的 package-level `casbin.Authorize`，避免与 `Enforce` 和 `Authorizer.Authorize` 形成重复入口。
- 将 `common/http/middleware.Auth()` 标记为废弃兼容入口或在确认无外部消费者后移除，并推荐调用方显式使用 `AuthWithTokenVersionValidator(..., nil)` 表达“不执行 token version 撤销校验”。
- 保持 user-service 当前认证、token version 校验、RBAC 授权行为和 HTTP 响应语义不变。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `shared-platform-primitives`: 收紧共享 HTTP 与安全 helper 的导出 API 约束，要求重复简写入口以显式推荐入口、废弃标记或移除策略治理。

## Impact

- 影响代码：`common/security/casbin/authorizer.go`、`common/security/casbin/authorizer_test.go`、`common/http/middleware/auth.go`、`common/http/middleware/auth_test.go`。
- 影响使用方：user-service 当前只生产调用 `commoncasbin.Enforce` 和 `AuthWithTokenVersionValidator`，预期不需要改动业务路由；若存在仓库外消费者调用被废弃或移除的导出 helper，需要按迁移说明调整。
- API 影响：`common` 导出符号可能出现废弃标记；若最终移除 package-level `casbin.Authorize` 或 `middleware.Auth`，属于共享模块公共 API 收紧，必须在任务中显式验证仓库内调用点。
- 不影响数据库 schema、OpenAPI、部署资产、观测指标或 Redis key schema。
