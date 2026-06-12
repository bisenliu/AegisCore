# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本次只新增 RBAC 数据模型。
- [x] 阅读 `user-service/ent/README.md` 和现有 `user-service/ent/schema/user.go`，对齐 Ent schema 风格。
- [x] 检查 `user-service/migrations/` 当前 migration 和 `atlas.sum`，确认新增 migration 命名为 `add-rbac-data-model` 或等价清晰名称。
- [x] 确认 `user-service/internal/features/role` 和 `user-service/internal/features/permission` 当前仍只是 skeleton，本变更不扩展 feature 业务代码。

## Ent Schema

- [x] 新增 `user-service/ent/schema/role.go`。
- [x] 新增 `user-service/ent/schema/permission.go`。
- [x] 新增 `user-service/ent/schema/userrole.go`。
- [x] 新增 `user-service/ent/schema/rolepermission.go`。
- [x] 在 `Role` schema 中定义 `role_id`、`name`、`description`、`active`、`is_system`、`created_at`、`updated_at`。
- [x] 在 `Permission` schema 中定义 `permission_id`、`name`、`description`、`module`、`http_method`、`path_template`、`active`、`is_system`、`created_at`、`updated_at`。
- [x] 在 `UserRole` schema 中定义 `user_id`、`role_id`、`created_at`，并通过 edge 关联 `User` 和 `Role`。
- [x] 在 `RolePermission` schema 中定义 `role_id`、`permission_id`、`created_at`，并通过 edge 关联 `Role` 和 `Permission`。
- [x] 为 `permissions.http_method + permissions.path_template` 添加唯一约束。
- [x] 为 `user_roles.user_id + user_roles.role_id` 添加唯一约束。
- [x] 为 `role_permissions.role_id + role_permissions.permission_id` 添加唯一约束。
- [x] 确认 `roles` 不包含 `code` 字段。
- [x] 确认 `permissions` 不包含 `code`、`source`、`resource`、`action` 字段。
- [x] 确认 `role_permissions` 不包含 `granted_by` 字段。

## Ent Generation

- [x] 运行 Ent 生成：

```bash
make generate
```

- [x] 审查 `user-service/ent/` 生成代码，只确认生成结果，不手写修改生成文件。
- [x] 确认生成代码中存在 `Role`、`Permission`、`UserRole`、`RolePermission` 相关 client、query、mutation 和 edge。

## Migration

- [x] 生成 Atlas migration：

```bash
make migrate-diff name=add-rbac-data-model
```

- [x] 审查新增 SQL migration 包含 `roles` 表。
- [x] 审查新增 SQL migration 包含 `permissions` 表。
- [x] 审查新增 SQL migration 包含 `user_roles` 表。
- [x] 审查新增 SQL migration 包含 `role_permissions` 表。
- [x] 确认 `user_roles.user_id` FK 指向 `users.id`，不是 `users.user_id`。
- [x] 确认 `user_roles.role_id` FK 指向 `roles.id`。
- [x] 确认 `role_permissions.role_id` FK 指向 `roles.id`。
- [x] 确认 `role_permissions.permission_id` FK 指向 `permissions.id`。
- [x] 确认 FK 删除行为为 restrict/no action 语义；如需手工调整 SQL，重新计算 migration hash。
- [x] 确认唯一约束覆盖字段正确。
- [x] 确认 migration 没有创建 `casbin_rules` 表。
- [x] 确认 migration 没有修改现有 `users` 表语义。

## Guardrails

- [x] 不新增角色管理 HTTP API。
- [x] 不新增权限管理 HTTP API。
- [x] 不新增 Casbin dependency、model、adapter、enforcer provider 或 middleware。
- [x] 不修改 JWT、登录、刷新、登出、token version 或 session 行为。
- [x] 不新增 seed data。
- [x] 不新增 `casbin_rules` 表。
- [x] 不修改现有外部 HTTP API。
- [x] 不在 `common/` 中新增 RBAC 业务 helper。
- [x] 不新增横向 `internal/rbac`、`internal/authorization`、`internal/service` 或 `internal/repository`。

## Permission List Decision

- [x] 记录第一阶段权限列表采用手动维护，不从 Gin route 或 Swagger 自动写入数据库。
- [x] 如需要初始系统权限，另开变更设计 seed 或初始化策略，不放入本数据模型变更。
- [x] 如需要自动化辅助，另开变更设计只读漂移检测工具，不直接生成生产权限事实。

## Formatting

- [x] 格式化新增 Ent schema：

```bash
gofmt -w user-service/ent/schema/role.go user-service/ent/schema/permission.go user-service/ent/schema/userrole.go user-service/ent/schema/rolepermission.go
```

## Verification

- [x] 校验迁移：

```bash
make migrate-validate
```

- [x] 运行用户服务测试：

```bash
cd user-service && go test ./...
```

- [x] 运行结构检查：

```bash
test -f user-service/ent/schema/role.go
test -f user-service/ent/schema/permission.go
test -f user-service/ent/schema/userrole.go
test -f user-service/ent/schema/rolepermission.go
rg -n "roles|permissions|user_roles|role_permissions" user-service/migrations
```

- [x] 运行禁止项检查：

```bash
rg -n "casbin_rules|granted_by" user-service/ent/schema user-service/migrations
rg -n "source|resource|action|code" user-service/ent/schema user-service/migrations
```

- [x] 检查 `git diff -- user-service/ent/schema user-service/ent user-service/migrations`，确认变更只覆盖 RBAC 数据模型和生成物。

## Review Notes

- [x] 确认 `roles` 第一阶段没有 `code`。
- [x] 确认 `permissions` 第一阶段没有 `code`、`source`、`resource`、`action`。
- [x] 确认 `role_permissions` 第一阶段没有 `granted_by`。
- [x] 确认 `user_roles` 通过 `users.id` 关联用户。
- [x] 确认后续 Casbin role subject 使用 `roles.role_id` 的设计约定已写入 design。
- [x] 确认权限列表手动维护的设计决策已写入 proposal 和 design。
