## 1. Ent schema 索引实现

- [x] 1.1 更新 `user-service/ent/schema/role.go`，为角色列表和授权回源补充 `active, role_id` 与 `is_system, role_id` 复合索引。
- [x] 1.2 更新 `user-service/ent/schema/permission.go`，为权限列表补充 `active, permission_id`、`module, permission_id`、`http_method, permission_id` 与 `is_system, permission_id` 复合索引，并保留现有路由身份唯一索引。
- [x] 1.3 更新 `user-service/ent/schema/userrole.go` 和 `rolepermission.go`，分别补充 `role_id, user_id` 与 `permission_id, role_id` 反向复合索引。
- [x] 1.4 处理 `User.nickname` contains 查询索引策略，保留业务查询语义，并确定普通 B-tree 与 PostgreSQL trigram GIN 索引的最终组合。

## 2. 生成物和 migration

- [x] 2.1 运行 `make user-service-generate`，更新 Ent 生成代码，不手写 `user-service/ent/` 生成物。
- [x] 2.2 运行 `make user-service-migrate-diff name=optimize-ent-indexes`，生成 Atlas SQL migration。
- [x] 2.3 审查新 migration，确认索引名称、重复索引、`pg_trgm` extension 和 `users.nickname` trigram GIN 索引符合设计；如 Atlas 自动 diff 未生成 trigram 索引，则在 migration 中补充 PostgreSQL 专用 SQL。
- [x] 2.4 运行 `make user-service-migrate-validate`，确认 migration 目录和 `atlas.sum` 有效。

## 3. 测试和验证

- [x] 3.1 运行相关 Go 测试，至少覆盖 `user-service/internal/features/user/...`、`role/...`、`permission/...` 和 Ent schema 相关测试。
- [x] 3.2 运行 `make user-service-architecture-lint`，确认 OpenSpec 和中文文档规则通过。
- [x] 3.3 运行 `make lint` 和 `make verify`；如本地依赖导致无法完成，记录失败原因和已完成的替代验证。
- [x] 3.4 审查 `git diff`，确认只包含 Ent schema、生成物、migration、OpenSpec change artifacts 和必要测试/文档变更，没有 OpenAPI、部署资产或无关生成物 drift。
