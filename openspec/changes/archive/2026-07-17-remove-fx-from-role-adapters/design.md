## Context

role feature 当前在 `user-service/internal/features/role/infrastructure/postgres/` 的 `RoleStore`、`RolePermissionStore`、`UserRoleStore` constructor 中定义 `fx.In` 参数结构，并通过 `name:"primary_db"` 直接表达服务级具名 Ent client 依赖。该写法让 role infrastructure package 承载 Fx/Dig metadata，导致普通 Go 装配和 store 单元测试必须知道 DI 容器形态，也弱化了 “application/domain 框架无关、composition 承载 DI metadata” 的边界。

本变更只收敛 role adapter 构造边界，不改变角色、角色权限、用户角色、RBAC seed、Casbin policy sync 或 HTTP API 行为。`rbac-access-control` 主规格已有 RBAC 分层与组合边界要求，本 change 将其扩展到 role infrastructure constructor 和架构 lint。

## Goals / Non-Goals

**Goals:**

- 将 `RoleStore`、`RolePermissionStore`、`UserRoleStore` 的 constructor 改为普通 Go 参数：显式 `*ent.Client`，以及需要时显式消费侧窄 port。
- 保持 `PermissionLookup` 作为 role application 消费侧窄 port 的 adapter，不把 permission feature infrastructure 或宽接口泄漏到 role application。
- 让 `user-service/internal/features/role/fx.go` 作为唯一允许携带 Fx metadata 的 role feature production composition 边界。
- 更新测试，使 role infrastructure store 可不经 Fx/Dig 直接构造。
- 增加架构检查，阻止 role feature 的 domain、application、infrastructure 和 transport 生产包重新导入 Fx/Dig。

**Non-Goals:**

- 不删除 role feature 的 Fx module，也不把 role feature 从服务级 Fx graph 中移除。
- 不改变 HTTP route、request/response、OpenAPI 注解或外部 API 行为。
- 不修改 Ent schema、Atlas migration、RBAC baseline、Redis key、Casbin model/policy、watcher、initial load 或 Redis policy sync 生命周期。
- 不在 `common`、`internal/shared` 或 `internal/integration` 新增 role 专用 helper。
- 不保留旧 constructor、兼容 wrapper 或双路径装配。

## Decisions

- `RoleStore` 和 `UserRoleStore` constructor 直接接收 `client *ent.Client`。
  备选方案是保留 `RoleStoreParams` 但移除 `fx.In`；该方案仍会留下仅为装配便利存在的参数结构，不如显式参数直接表达依赖。

- `RolePermissionStore` constructor 直接接收 `client *ent.Client`，不为当前 store 操作额外引入 `PermissionLookup` 成员。
  备选方案是把 `PermissionLookup` 注入到 `RolePermissionStore` 中并在 store 内调用；该方案会把 application 编排语义下沉到 infrastructure，并改变现有事务内权限锁定逻辑，因此不采用。`PermissionLookup` 继续由 application service 作为消费侧 port 显式接收。

- Fx/Dig 具名资源适配只放在 `user-service/internal/features/role/fx.go`。
  `fx.go` 通过小型 provider 或 `fx.Annotate` 的参数标签把 `primary_db` Ent client 投影为普通 constructor 调用，再 `fx.As` 到 role application port。这样可以保留生产 Fx graph 的具名资源语义，同时避免 infrastructure package 导入 Fx。

- 架构 lint 使用源代码搜索或现有 lint 框架增加 role feature 禁止规则。
  检查范围覆盖 `user-service/internal/features/role` 下 Go production 文件，排除 `fx.go`、`fx_test.go` 和测试文件；匹配 `go.uber.org/fx`、`go.uber.org/dig`、`fx.In`、`fx.Out`、`dig.In`、`dig.Out`。这样能防止 domain、application、infrastructure 和 transport 生产包重新依赖 DI metadata。

- 测试迁移到新 constructor 签名，不引入测试专用生产接口。
  store 测试直接传入 Ent test client；composition 测试继续验证 `role.Module` 能构图并提供 application port。

## Risks / Trade-offs

- Go constructor API 不兼容 → 本仓库内调用点必须一次性更新，且不保留兼容 wrapper；通过 `go test ./internal/features/role/... -count=1` 捕获遗漏。
- Fx graph 参数标签遗漏导致生产启动失败 → 在 `fx_test.go` 或服务级 graph 测试中保留 role module 构图验证，并通过架构 lint 限制 metadata 只在 composition 边界。
- 架构 lint 误伤测试或 composition 文件 → 检查规则明确排除 `*_test.go` 和 `fx.go`，并用验收中的 `rg` 命令确认生产包无 Fx/Dig import。
- 只改 constructor 可能被误解为行为变更 → specs 和 tasks 明确 HTTP API、OpenAPI、Ent schema、migration、watcher、Casbin 和 Redis policy sync 均不变化。

## Migration Plan

1. 修改 role infrastructure store constructor 签名并移除 `go.uber.org/fx` import、Params 类型和 `name:"primary_db"` tag。
2. 修改 `user-service/internal/features/role/fx.go`，在 composition 层适配具名 `primary_db` Ent client 到普通 constructor，并继续 `fx.As` 到 application port。
3. 更新 role infrastructure、seed、composition 和相关 application 测试的 constructor 调用。
4. 增加或调整 `user-service-architecture-lint` 规则，禁止 role 非 composition 生产包导入 Fx/Dig。
5. 执行验收命令：role feature 测试、Fx/Dig import 搜索、`openspec validate remove-fx-from-role-adapters`、`make user-service-architecture-lint`，暂存预期变更后执行 `make lint` 和 `make verify`。

回滚方式是恢复本 change 对 constructor、composition、测试、架构 lint 和 specs 的修改；由于不涉及数据库、外部 API、部署资产或持久化数据，回滚不需要 migration 或数据修复。

## Open Questions

- 无。
