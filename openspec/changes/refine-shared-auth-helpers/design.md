## Context

`common/security/casbin` 当前同时暴露 package-level `Enforce`、package-level `Authorize` 和 `Authorizer.Authorize`。其中 `Enforce` 返回 Casbin 原始允许结果，user-service 权限引擎在授权热路径中实际使用；`Authorize` 仅在 `Enforce` 基础上把 `allowed=false` 转为 `ErrDenied`，主要通过 `Authorizer.Authorize` 形成 error-only 语义。

`common/http/middleware.Auth()` 当前只是 `AuthWithTokenVersionValidator(log, jwtService, cfg, nil)` 的简写，仓库内没有生产调用方。user-service 路由已经显式使用 `AuthWithTokenVersionValidator` 并传入 token version validator，以支撑登出、强制改密和 token version 撤销语义。

这次变更只治理共享 helper 的 API 形态，不改变认证解析、token version 校验、Casbin policy、RBAC subject/object/action、HTTP 状态码、响应 envelope、数据库 schema 或部署资产。

## Goals / Non-Goals

**Goals:**

- 保留 `commoncasbin.Enforce` 作为需要原始授权 bool 的推荐入口。
- 保留 `Authorizer.Authorize` 的“拒绝即错误”语义，并让它直接基于 `Enforce` 实现，减少 package-level 重复入口。
- 对 `common/http/middleware.Auth()` 采取明确的兼容治理策略，优先标记 `Deprecated`，并在仓库内推荐显式调用 `AuthWithTokenVersionValidator(..., nil)`。
- 更新测试，确认 user-service 当前认证和 RBAC 授权行为不变。

**Non-Goals:**

- 不调整 `redisScore` 与 `redisScoreFloat`，因为二者分别满足 Redis 字符串参数和 `float64` score 的类型需求。
- 不修改 token version validator、JWT claims、refresh session、Casbin policy loader、policy sync 或 route diff 行为。
- 不新增 `common`、`internal/shared` 或 feature 边界外的新 helper。
- 不更新 OpenAPI、Ent schema、Atlas migration、部署清单或观测资产。

## Decisions

1. `casbin.Enforce` 继续保留为 package-level 函数。

   选择原因：它提供通用三元组校验和错误包装，且 user-service 权限引擎需要 `bool` 结果来组合多个角色的授权判断。备选方案是只保留 `Authorizer` 类型方法，但会迫使调用方构造额外对象，降低热路径表达清晰度。

2. `Authorizer.Authorize` 保留，但不再依赖 package-level `Authorize`。

   选择原因：`Authorizer.Authorize` 符合 common HTTP Casbin middleware 需要的 error-only 接口；将拒绝转换逻辑内联在方法内，可以保留语义并减少一个导出 package-level wrapper。备选方案是删除 `Authorizer.Authorize`，但会影响 `common/http/middleware/casbin.go` 的接口形态，收益不足。

3. package-level `casbin.Authorize` 在实现前再次确认调用点；若只有本包测试引用，则移除并迁移测试到 `Authorizer.Authorize`。

   选择原因：该函数与 `Enforce` 的差异很小，且没有仓库内生产调用。备选方案是仅标记废弃，但 `common/security/casbin` 当前属于仓库内共享模块，若确认没有使用方，直接移除能更清晰地收紧 API。

4. `middleware.Auth()` 优先标记 `Deprecated`，不在本变更中直接删除。

   选择原因：该函数是 `common/http/middleware` 的导出 API，虽然仓库内没有调用方，但外部或未来服务消费者可能依赖。废弃标记能让文档和 IDE 给出迁移提示，同时保持兼容。备选方案是直接删除，但对共享模块公共 API 更激进。

5. 规格只修改 `shared-platform-primitives`。

   选择原因：本变更治理的是 `common/http` 和 `common/security` 共享 helper 的 API 面。`rbac-access-control` 的授权业务行为保持不变，因此不修改 RBAC 主规格。

## Risks / Trade-offs

- [Risk] 仓库外代码仍调用 package-level `casbin.Authorize` 或 `middleware.Auth()` → Mitigation：`middleware.Auth()` 先废弃不删除；`casbin.Authorize` 移除前用 `rg` 验证仓库内调用点，并在任务说明中记录迁移方式。
- [Risk] 将 `Authorizer.Authorize` 改为直接实现时遗漏 nil authorizer 或 nil enforcer 处理 → Mitigation：保留现有 `ErrNotConfigured` 语义，并覆盖 nil authorizer、nil enforcer、denied 和 enforcer error 测试。
- [Risk] 认证中间件测试误删后降低 token version 覆盖 → Mitigation：保留 `AuthWithTokenVersionValidator` 的现有测试，新增或调整测试只验证废弃 wrapper 委托行为。
- [Risk] API 收紧被误解为认证或 RBAC 行为变更 → Mitigation：实现任务必须明确不修改路由挂载、JWT parse、token version validator、Casbin policy loader 和 HTTP response helper。

## Migration Plan

实现时先确认 `commoncasbin.Authorize` 和 `middleware.Auth` 的仓库内调用点。仓库内若存在调用，迁移为 `commoncasbin.Enforce` 加显式 denied 处理、`Authorizer.Authorize`，或 `AuthWithTokenVersionValidator(..., nil)`。

部署不需要数据库 migration 或配置变更。回滚方式是恢复被移除的 package-level wrapper，或移除 `Deprecated` 注释；由于 user-service 生产路径不依赖这些 wrapper，回滚不涉及数据或运行时状态。

验证方式包括运行相关 Go 测试、架构 lint，并用 `rg` 确认推荐入口和废弃入口的调用状态。

## Open Questions

- package-level `casbin.Authorize` 是否需要保留一个版本周期并标记 `Deprecated`，取决于是否将 `common` 视为已有仓库外消费者的稳定公共 API。
