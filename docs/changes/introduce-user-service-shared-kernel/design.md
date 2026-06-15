# Design

## Shared Boundary

`user-service/internal/shared` 是用户服务内共享业务内核，不是跨服务公共库，也不是 feature 外的杂物目录。只有满足以下条件的能力可以进入：

- 已被至少两个用户服务 feature 真实消费。
- 表达稳定业务规格、纯类型、值对象、系统内置规格或少量无副作用判断方法。
- 不能归入跨服务无业务语义的 `common`。
- 新增子包时必须在 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 说明 owner、消费方、准入理由和禁止事项。

`internal/shared` 禁止导入或承载：

- Gin、Ent、Redis、SQL、Fx provider。
- controller、transport DTO、Swagger DTO、store port、application use case。
- feature infrastructure adapter、feature application service、HTTP response helper。
- 配置读取、日志副作用、外部系统调用、数据库或缓存访问。

初始 owner 规则：

- `shared/identity` owner 是 user 与 auth 的共同业务内核；user feature 负责资料生命周期入口，auth feature 负责认证消费，变更需同时验证两侧。
- `shared/rbacbaseline` owner 是 role 与 permission 的共同 RBAC 系统规格；permission 仍拥有权限目录生命周期、Casbin policy loader 和 route diff，role 仍拥有角色生命周期和绑定 use case。

## Identity Package

新增：

```text
user-service/internal/shared/identity
```

包内承载：

- `type UserStatus int64`
- `UserStatusNormal`
- `UserStatusDisabled`
- `UserStatusMustChangePassword`
- `IsValid`
- `AllowedValues`
- `CanLogin`
- `RequiresPasswordChange`
- `UnmarshalText`
- `UnmarshalJSON`

迁移后全仓直接使用 `identity.UserStatus`。不在 `user/domain` 保留 type alias、兼容常量或 wrapper 方法，避免新旧入口长期并存。

`user/domain.User` 的 `Status` 字段改为 `identity.UserStatus`。`User.CanLogin`、`User.RequiresPasswordChange` 和 `User.CanChangePassword` 可以继续作为实体便捷方法，但内部只委托 `identity`。

`auth/domain.UserCredential` 和 `auth/domain.UpdateCredentialsInput` 的 `Status` 字段改为 `identity.UserStatus`。Auth application 不再导入 user domain；凭据状态判断统一走 identity。

Ent schema 默认值测试改为校验 `defaultUserStatus == int64(identity.UserStatusNormal)`。Ent generated code 不手改；若 schema 注释需要更新，只改 `ent/schema` 后运行生成。

## RBAC Baseline Package

新增：

```text
user-service/internal/shared/rbacbaseline
```

包内承载：

- `SuperAdminRoleID`
- `RoleSpec`
- `PermissionSpec`
- `RolePermissionSpec`
- `DefaultRoles`
- `DefaultPermissions`
- `DefaultRolePermissions`

迁移后删除：

```text
user-service/internal/features/permission/application/rbacbaseline
```

不保留迁移兼容层。所有调用方直接导入 `internal/shared/rbacbaseline`。

依赖方向：

```text
internal/shared/rbacbaseline
  ↑
  ├─ role/application/seed
  ├─ permission/infrastructure/casbin
  └─ permission/transport/http route scanner tests and route diff support
```

Permission feature 继续拥有权限目录生命周期、有效权限查询、只读 route diff、authorization wrapper、Gin RBAC middleware 和 Casbin adapter。Shared baseline 只表达系统初始规格，不做目录写入、不加载 policy、不访问 store。

Role feature 继续通过 permission application port 校验权限是否存在且启用。该依赖是角色绑定权限 use case 的业务协作，不是共享模型，暂不搬迁。

## Architecture Lint

新增脚本：

```text
user-service/scripts/architecture-lint.sh
```

脚本使用 shell 与 `go list`/`rg`/`git diff` 组合检查以下规则：

- Feature 间 import 只能走白名单；例如 role seed 可消费 shared baseline，但 auth 不得导入 user domain。
- `transport/http` 的 request/response DTO 不得被其他 transport、application、domain、infrastructure 或其他 feature 导入。
- `internal/shared` 不得导入 `internal/features`、Gin、Ent、Redis、SQL、Fx、common HTTP response、runtime config/logger/datastore provider。
- `permission/application/rbacbaseline` 不存在。
- 全仓不再出现 `userdomain.UserStatus` 或旧 RBAC baseline import。
- `swagger-generate` 后无文档 drift。
- Ent schema 或生成相关文件变更后，生成代码不能有未提交 drift。

`Makefile` 新增：

```make
architecture-lint: ## Run user-service architecture boundary checks.
	cd $(USER_SERVICE_DIR) && ./scripts/architecture-lint.sh

verify: lint architecture-lint test swagger-generate ## Run full local verification.
	git diff --exit-code
```

`verify` 将 Swagger drift 与 generated code drift 都收敛到最终 `git diff --exit-code`。若生成命令修改产物，开发者必须提交对应变更。

## CI

CI 增加一个明确 verification job 或扩展现有 job，按顺序执行：

```bash
make lint
make architecture-lint
make test
make swagger-generate
git diff --exit-code
```

已有独立安全扫描、race、coverage 和 Docker build job 可以保留。`lint.yml` 继续作为快速 lint 门禁；主 `ci.yml` 需要覆盖架构 lint 和生成产物 drift。

## Tests

需要新增或迁移测试：

- `shared/identity`：状态合法性、allowed values、login 判断、must-change-password 判断、text/json 解析。
- `shared/rbacbaseline`：角色 ID 唯一、权限 ID 唯一、route identity 唯一、默认绑定引用存在、超级管理员角色存在。
- User domain 测试更新为使用 `identity.UserStatus`。
- Auth credentials、command 和 PostgreSQL adapter 测试更新为使用 `identity.UserStatus`。
- Role seed、permission route scanner 和 Casbin policy tests 更新为使用 `shared/rbacbaseline`。

## Migration Order

1. 更新架构文档，明确 `internal/shared` 准入规则。
2. 新增 `shared/identity`，迁移用户状态定义和测试。
3. 批量更新 user/auth 引用，删除旧 user domain 状态定义。
4. 新增 `shared/rbacbaseline`，迁移 baseline 和测试。
5. 批量更新 role/permission 引用，删除旧 permission baseline 包。
6. 新增 architecture lint 脚本、Makefile target 和 CI gate。
7. 运行 `gofmt`、`make verify`，修复 drift。
