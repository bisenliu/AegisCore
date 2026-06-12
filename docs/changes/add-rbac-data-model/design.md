# Design

## Overview

本变更在 `user-service` 内建立 RBAC 持久化事实模型：

```text
users(id) ──< user_roles >── roles(id) ──< role_permissions >── permissions(id)
```

其中：

- `users.user_id` 继续作为 HTTP API 对外用户 ID。
- `users.id` 作为服务内部 FK，被 `user_roles.user_id` 引用。
- `roles.role_id` 是角色稳定业务 ID，也是后续 Casbin role subject。
- `permissions.permission_id` 是权限稳定业务 ID。
- `permissions.http_method + permissions.path_template` 唯一标识 URL + method 权限。
- 不创建 `casbin_rules`，后续 Casbin policy 从规范化业务表加载。

## Feature Ownership

`role` 和 `permission` 目前是 future feature skeleton。本变更新增的是服务级 Ent schema 和数据库 migration，不新增 feature application、domain、transport 或 infrastructure 代码。

原因：

- 当前只需要数据模型，为后续 role/permission feature 实现提供存储基础。
- 不存在角色管理或权限管理 HTTP API，因此不创建 controller、DTO、use case 或 feature-local adapter。
- Ent schema 归 `user-service/ent/schema` 管理，迁移归 `user-service/migrations` 管理。

后续新增角色或权限业务行为时，仍按 feature 分层扩展：

- 角色生命周期、角色分配、角色查询放入 `user-service/internal/features/role`。
- 权限定义、权限查询、权限清单维护放入 `user-service/internal/features/permission`。
- 授权中间件和 Casbin runtime wiring 需要单独设计，不能混入本变更。

## Table Design

### `roles`

| 字段 | 说明 |
|---|---|
| `id` | 内部自增主键，用于 FK。 |
| `role_id` | 稳定 UUID 业务 ID，后续作为 Casbin role subject。 |
| `name` | 角色显示名称，最大 128。 |
| `description` | 角色说明，最大 512，默认空字符串。 |
| `active` | 是否启用，默认 `true`。 |
| `is_system` | 是否系统角色，默认 `false`。 |
| `created_at` | 创建时间。 |
| `updated_at` | 更新时间。 |

第一阶段不增加 `code`。系统角色通过固定 `role_id` 和 `is_system` 标记识别，超级管理员角色后续通过固定 super admin `role_id` 初始化。

### `permissions`

| 字段 | 说明 |
|---|---|
| `id` | 内部自增主键，用于 FK。 |
| `permission_id` | 稳定 UUID 业务 ID。 |
| `name` | 权限显示名称，最大 128。 |
| `description` | 权限说明，最大 512，默认空字符串。 |
| `module` | 权限所属业务模块，最大 64。 |
| `http_method` | HTTP method，最大 16。 |
| `path_template` | URL path template，最大 512。 |
| `active` | 是否启用，默认 `true`。 |
| `is_system` | 是否系统权限，默认 `false`。 |
| `created_at` | 创建时间。 |
| `updated_at` | 更新时间。 |

约束：

- `permission_id` 唯一。
- `(http_method, path_template)` 唯一，约束名为 `permissions_method_path_unique`。

第一阶段不增加 `code`、`source`、`resource` 或 `action`。权限事实以 `http_method + path_template` 为核心，后续 Casbin `p` policy 可由 active permission 映射得到。

### `user_roles`

| 字段 | 说明 |
|---|---|
| `id` | 内部自增主键。 |
| `user_id` | 引用 `users.id`，不是 `users.user_id`。 |
| `role_id` | 引用 `roles.id`。 |
| `created_at` | 绑定创建时间。 |

约束：

- `user_id` FK 使用 `ON DELETE RESTRICT`。
- `role_id` FK 使用 `ON DELETE RESTRICT`。
- `(user_id, role_id)` 唯一，约束名为 `user_roles_user_role_unique`。

后续 Casbin `g` policy 可从 `users.user_id -> roles.role_id` 映射得到；数据库内仍使用 `users.id` 保持 FK 简洁和一致。

### `role_permissions`

| 字段 | 说明 |
|---|---|
| `id` | 内部自增主键。 |
| `role_id` | 引用 `roles.id`。 |
| `permission_id` | 引用 `permissions.id`。 |
| `created_at` | 绑定创建时间。 |

约束：

- `role_id` FK 使用 `ON DELETE RESTRICT`。
- `permission_id` FK 使用 `ON DELETE RESTRICT`。
- `(role_id, permission_id)` 唯一，约束名为 `role_permissions_role_permission_unique`。

第一阶段不增加 `granted_by`，避免在没有管理 API 和审计模型时引入不完整审计语义。

## Ent Schema Mapping

新增 schema 应遵循现有 `User` schema 风格：

- `id` 使用 Ent 默认自增主键或显式 `field.Int64("id")`，以生成 PostgreSQL `bigserial`。
- UUID 业务 ID 使用 `field.UUID(..., uuid.UUID{}).Default(uuid.New).Unique().Immutable()`。
- 字符串字段设置 `NotEmpty()` 和 `MaxLen(...)`，`description` 使用 `Default("")`。
- 布尔字段设置默认值。
- `created_at` 使用 `Default(time.Now).Immutable()`。
- `updated_at` 使用 `Default(time.Now).UpdateDefault(time.Now)`。
- join 表只需要 `created_at`，不需要 `updated_at`。

Edges：

- `Role` has many `UserRole`。
- `Role` has many `RolePermission`。
- `Permission` has many `RolePermission`。
- `UserRole` belongs to one `User` and one `Role`。
- `RolePermission` belongs to one `Role` and one `Permission`。

应尽量让 FK 字段名与目标 SQL 一致：

- `user_roles.user_id`
- `user_roles.role_id`
- `role_permissions.role_id`
- `role_permissions.permission_id`

如果 Ent edge 默认命名无法生成期望列名，需要在 edge 上使用 `Field(...)` 绑定显式字段。

Indexes：

- `roles.role_id` 唯一。
- `permissions.permission_id` 唯一。
- `permissions.http_method + permissions.path_template` 唯一。
- `user_roles.user_id + user_roles.role_id` 唯一。
- `role_permissions.role_id + role_permissions.permission_id` 唯一。

## Migration Strategy

按现有流程执行：

```bash
make generate
make migrate-diff name=add-rbac-data-model
make migrate-validate
```

生成 migration 后需要人工审查：

- 四张表存在且名称正确。
- FK 都指向内部 `id` 列。
- FK 删除策略为 `ON DELETE RESTRICT` 或等价 restrict/no action 语义；若 Atlas 生成默认 `NO ACTION`，需要确认是否满足项目约束，必要时手工调整并重新 hash。
- 唯一约束存在且覆盖字段正确。
- 没有生成 `casbin_rules`。
- 没有向 `users` 表添加业务字段。

如果手动调整 SQL，需要在 `user-service/` 下执行：

```bash
atlas migrate hash --dir file://migrations
./scripts/migrate-validate.sh
```

## Permission List Maintenance

第一阶段权限列表应手动维护，而不是自动生成。

推荐路径：

- 本变更只创建表结构，不创建 seed data。
- 后续单独变更定义系统角色和系统权限初始化策略。
- 权限初始化可以采用 Atlas seed migration、独立 CLI command 或后台管理 API，但需要单独设计幂等、删除策略、命名和审计。

不推荐在第一阶段自动扫描 Gin routes 写入 `permissions`：

- Gin route 缺少权限名称、模块、系统权限标记等业务字段。
- 一条业务权限可能覆盖多条 route，自动生成会导致权限粒度失控。
- 某些 route 可能是公开接口、健康检查或 Swagger，不应进入权限表。
- 自动写库需要处理删除、重命名、禁用和历史权限迁移，复杂度超出数据模型阶段。

可在后续做只读校验工具：扫描 route 或 Swagger 生成候选权限，与手动维护清单对比，输出差异报告，不直接修改数据库。

## Casbin Integration Contract

本变更不接入 Casbin，但保留后续映射约定：

- Casbin subject 可使用外部用户 ID，即 `users.user_id` 字符串。
- Casbin role 使用 `roles.role_id` 字符串，不使用 role name 或 code。
- Casbin object 使用 `permissions.path_template`。
- Casbin action 使用 `permissions.http_method`。
- 只加载 `active = true` 的角色、权限和绑定。

后续 Casbin loader 需要通过 join 查询生成：

- `g, <user_id>, <role_id>`：来自 `users` + `user_roles` + `roles`。
- `p, <role_id>, <path_template>, <http_method>`：来自 `roles` + `role_permissions` + `permissions`。

## Non-Goals

- 不定义 Casbin model 文件。
- 不新增 Casbin dependency。
- 不创建 Casbin enforcer provider。
- 不注册授权中间件。
- 不读取、缓存或热更新 policy。
- 不新增角色或权限 HTTP API。
- 不新增 seed data。
- 不改变 auth/user 当前行为。

## Verification Strategy

实现后运行：

```bash
make generate
make migrate-validate
cd user-service && go test ./...
```

结构检查：

```bash
test -f user-service/ent/schema/role.go
test -f user-service/ent/schema/permission.go
test -f user-service/ent/schema/userrole.go
test -f user-service/ent/schema/rolepermission.go
rg -n "casbin_rules|granted_by|source|resource|action|code" user-service/ent/schema user-service/migrations
rg -n "roles|permissions|user_roles|role_permissions" user-service/migrations
```

注意：`action` 或 `code` 可能出现在迁移工具注释或无关上下文中；检查时以表字段为准。

## Risks And Mitigations

### Ent edge column drift

Risk: Ent 默认 edge 命名生成不符合预期的 FK 列名。

Mitigation: 在 join schema 中显式定义 FK field，并用 edge `Field(...)` 绑定；审查生成 migration。

### Premature Casbin coupling

Risk: 数据模型变更顺手引入 Casbin dependency、model、middleware 或 runtime provider。

Mitigation: 本变更验收明确禁止 Casbin 接入和 `casbin_rules` 表，仅保留后续映射约定。

### Permission auto-generation ambiguity

Risk: 从 route 自动写入权限表会把技术路由误当业务权限，导致粒度和审计失控。

Mitigation: 第一阶段手动维护权限事实；自动化只作为后续只读漂移检测工具。

### Delete behavior surprise

Risk: 删除用户、角色或权限时级联删除绑定，导致审计和授权事实丢失。

Mitigation: FK 使用 restrict/no action 语义，删除前需要显式处理绑定关系。
