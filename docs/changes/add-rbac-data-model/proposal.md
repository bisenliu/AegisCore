# Add RBAC data model

## What

为 `user-service` 新增 RBAC 规范化数据库模型，作为后续角色、权限、用户角色绑定、角色权限绑定和 Casbin policy 加载的业务事实来源。

本变更只建立持久化数据结构，不接入 Casbin runtime，不实现授权中间件，也不新增角色或权限管理 HTTP API。

目标表：

- `roles`
- `permissions`
- `user_roles`
- `role_permissions`

目标 Ent schema：

- `user-service/ent/schema/role.go`
- `user-service/ent/schema/permission.go`
- `user-service/ent/schema/userrole.go`
- `user-service/ent/schema/rolepermission.go`

## Why

后续 Casbin 集成需要稳定、可审计、可迁移的业务事实来源。直接依赖 `casbin_rules` 或只在代码中维护 policy，会让角色、权限、用户绑定和权限绑定缺少清晰所有权，也不利于管理端 API、初始化脚本、审计和后续缓存加载。

先引入规范化 RBAC 表，可以让后续 Casbin adapter 从业务表生成 `p` 和 `g` policy：

- `permissions` 表保存 `http_method + path_template` 权限事实。
- `roles` 表保存角色事实，Casbin role subject 使用 `role_id`。
- `user_roles` 表保存用户和角色绑定，内部通过 `users.id` 做 FK。
- `role_permissions` 表保存角色和权限绑定。

这样后续接入 Casbin 时，Casbin 只做授权决策引擎，不成为业务权限事实的主存储。

## Scope

包括：

- 新增 `roles` 表。
- 新增 `permissions` 表。
- 新增 `user_roles` 表。
- 新增 `role_permissions` 表。
- 新增对应 Ent schema。
- 通过 Ent 重新生成 `user-service/ent/` 下生成代码。
- 新增 Atlas migration，并更新 `user-service/migrations/atlas.sum`。
- 保持现有 `users` 表语义不变。
- `user_roles` 内部通过 `users.id` 关联用户。
- 外部 API 继续使用 `users.user_id` 作为用户标识。
- Casbin role subject 的设计约定为 `roles.role_id`，不使用 `roles.code`。

不包括：

- 不实现角色管理 HTTP API。
- 不实现权限管理 HTTP API。
- 不接入 Casbin。
- 不实现授权中间件。
- 不修改 JWT、登录、刷新、登出、token version 或 session 行为。
- 不新增 role、permission 之外的业务 feature。
- 不创建 `casbin_rules` 表。
- 不修改现有外部 HTTP API。
- 不新增 `roles.code`。
- 不新增 `permissions.code`、`permissions.source`、`permissions.resource` 或 `permissions.action`。
- 不新增 `role_permissions.granted_by`。

## Data Shape

```sql
CREATE TABLE roles (
  id BIGSERIAL PRIMARY KEY,
  role_id UUID NOT NULL UNIQUE,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  is_system BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE permissions (
  id BIGSERIAL PRIMARY KEY,
  permission_id UUID NOT NULL UNIQUE,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  module VARCHAR(64) NOT NULL,
  http_method VARCHAR(16) NOT NULL,
  path_template VARCHAR(512) NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  is_system BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT permissions_method_path_unique UNIQUE (http_method, path_template)
);

CREATE TABLE user_roles (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT user_roles_user_role_unique UNIQUE (user_id, role_id)
);

CREATE TABLE role_permissions (
  id BIGSERIAL PRIMARY KEY,
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
  permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT role_permissions_role_permission_unique UNIQUE (role_id, permission_id)
);
```

## Permission List Strategy

第一阶段权限列表采用手动维护的业务数据，不做从 Gin route 或 Swagger 自动生成。

原因：

- 自动扫描 route 只能得到技术路由，不能表达权限名称、模块归属、是否系统权限、是否对外开放、是否需要合并多个 route 为一个业务权限等业务语义。
- 当前变更不实现管理 API、初始化器或 Casbin loader，自动生成会引入额外运行时边界和同步策略，超出数据模型阶段。
- 手动维护 `permissions` 可以把权限事实作为显式业务配置或 seed migration 管理，后续更容易审计和评审。

后续可以单独设计“权限清单校验工具”：从路由或 Swagger 生成候选清单，与数据库或 seed 文件比对，只报告缺失和漂移，不直接写入生产权限表。

## Acceptance Criteria

- `user-service/ent/schema` 下存在 `Role`、`Permission`、`UserRole`、`RolePermission` schema。
- Ent 生成代码编译通过。
- Atlas migration 包含 `roles`、`permissions`、`user_roles`、`role_permissions`。
- `roles` 表不包含 `code`。
- `permissions` 表不包含 `code`、`source`、`resource`、`action`。
- `role_permissions` 表不包含 `granted_by`。
- 不存在 `casbin_rules` 表。
- `user_roles` 通过 `users.id` 关联用户。
- `role_permissions` 通过 `permissions.id` 关联权限。
- `make migrate-validate` 或等价迁移校验通过。
- `user-service` 下 `go test ./...` 通过。
