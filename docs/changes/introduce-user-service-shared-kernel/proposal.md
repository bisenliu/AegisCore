# Introduce User Service Shared Kernel

## What

新增用户服务内的窄 `internal/shared` 共享内核边界，用于承载已经被多个 feature 真实消费、但不适合进入跨服务 `common` 的稳定业务规格。

本变更先开放两个子包：

- `user-service/internal/shared/identity`：用户状态、账号生命周期判断和相关解析能力。
- `user-service/internal/shared/rbacbaseline`：系统内置 RBAC 角色、权限和默认角色权限绑定规格。

同时补充架构 lint 门禁，确保 feature 间 import、HTTP DTO 泄漏、`internal/shared` 依赖、旧 RBAC baseline 包、Swagger drift 和 Ent generated code drift 都能被本地 `make verify` 与 CI 拦截。

## Why

当前用户状态定义位于 `user` feature domain，但 auth 登录、改密和凭据 adapter 也依赖这套生命周期判断。这让 auth feature 为了识别账号状态被迫导入 user domain，形成跨 feature 业务耦合。

当前系统 RBAC baseline 位于 `permission/application/rbacbaseline`，但 role seed、permission route diff、Casbin policy loader 都消费它。随着 role 与 permission 都需要同一份系统内置规格，baseline 已经不再只是 permission use case 的私有输入，而是用户服务内稳定业务内核。

`common` 不能承载用户服务业务语义；继续让一个 feature 持有另一个 feature 也要消费的核心规格，又会让依赖方向和 ownership 变得含混。因此需要一个明确、严格受限的 `internal/shared` 边界，并通过 lint 把准入规则自动化。

## Scope

包括：

- 在 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 明确 `internal/shared` 的准入规则、禁止事项、owner 和消费方登记要求。
- 新增 `internal/shared/identity`，迁移 `user/domain.UserStatus` 的核心枚举、解析和状态判断。
- 删除旧 `user/domain.UserStatus` 定义；全仓统一使用 `shared/identity.UserStatus`，不保留 alias 或兼容常量。
- 调整 user domain、user application、user HTTP DTO、auth domain、auth application 和 auth PostgreSQL adapter 使用 `shared/identity`。
- 新增 `internal/shared/rbacbaseline`，迁移系统角色、系统权限和默认绑定规格。
- 删除 `permission/application/rbacbaseline` 旧包；permission seeding、route diff、Casbin loader 和 role seed 统一消费 `shared/rbacbaseline`。
- 保留 role 通过 permission application port 校验权限存在且启用，不把该 use case 级依赖搬到 shared。
- 新增 `user-service/scripts/architecture-lint.sh`，接入 `make architecture-lint` 和 `make verify`。
- 在 CI 中执行 `make lint`、`make architecture-lint`、`make test`、`make swagger-generate` 和 `git diff --exit-code`。

## Non-Goals

- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不把 `internal/shared` 扩展为通用 helper、service、repository、port 或 provider 目录。
- 不把 Gin、Ent、Redis、SQL、Fx provider、controller、HTTP DTO、store port 或 use case 放入 shared。
- 不修改数据库 schema、Atlas migration、Redis key schema、HTTP API path 或响应信封契约。
- 不改变 Casbin subject/object/action 语义，不改变 `user:<user_uuid>`、`role:<role_uuid>` 或 route template 授权规则。
- 不搬迁 role -> permission application port 依赖。

## Acceptance Criteria

- `user-service/internal/shared/identity` 只包含用户状态、账号生命周期纯判断和无副作用解析能力。
- `user-service/internal/shared/rbacbaseline` 只包含系统内置 RBAC 规格和无副作用校验测试。
- 全仓无 `userdomain.UserStatus`、`permission/application/rbacbaseline` 或 feature 间状态/baseline import 残留。
- `internal/shared` 不导入 feature 包、Gin、Ent、Redis、SQL、Fx、HTTP response 或 runtime provider。
- `auth` 不再导入 `features/user/domain`。
- `role` 和 `permission` 统一导入 `internal/shared/rbacbaseline`。
- `make architecture-lint` 成功通过，并能覆盖本变更定义的边界检查。
- `make verify` 成功通过。
- CI 中包含并执行架构 lint、测试、Swagger drift 和 generated code drift 检查。
