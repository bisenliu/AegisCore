## Context

user-service 当前通过 Ent schema 定义 PostgreSQL 表结构，并通过 Atlas SQL migration 交付数据库结构变更。用户、角色、权限和 RBAC 绑定查询已经集中在 `user-service/internal/features/user|role|permission/infrastructure/postgres` 与 `permission/infrastructure/casbin` 中，访问模式包括 keyset 分页、按外部 UUID 查找、授权热路径用户角色回源、有效权限聚合、角色权限绑定维护和 Casbin policy 全量加载。

现有 schema 已覆盖 `users.user_id`、`users.username`、用户软删除和状态分页等索引，也覆盖 `permissions.permission_id`、`permissions(http_method, path_template)`、`roles.role_id`、`user_roles(user_id, role_id)` 和 `role_permissions(role_id, permission_id)`。缺口主要集中在角色/权限列表过滤 + keyset 排序、绑定表反向 FK 查询，以及昵称 contains 查询使用普通 B-tree 索引无法有效支持前置通配符的问题。

本变更属于数据库 schema 与性能支撑变更，不改变 HTTP API、业务语义、OpenAPI、Casbin 授权结果、RBAC seed 数据或部署资产。实现必须停留在 `user-service` Ent schema、生成代码和 migration 边界内，不把索引策略下沉到 `common`、feature application 或 `internal/shared`。

## Goals / Non-Goals

**Goals:**

- 为角色、权限和 RBAC 绑定表补充与当前查询模式匹配的最小必要索引。
- 为用户昵称 contains 查询提供 PostgreSQL trigram 索引，避免无效依赖普通 B-tree。
- 通过 Ent 生成和 Atlas migration 交付可审查的 schema 变更。
- 保持现有 API、错误语义、授权语义和发布流程不变。
- 通过 migration 校验、架构 lint、相关测试和 diff 审查确认变更安全。

**Non-Goals:**

- 不新增用户、角色、权限或绑定接口能力。
- 不改变角色名称、权限名称或用户名的唯一性语义。
- 不调整分页排序字段、过滤参数、Casbin subject/object/action 格式或 RBAC policy sync 机制。
- 不修改 `common` 模块、部署镜像、Kubernetes/Helm/Compose 资产或 OpenAPI 生成物。
- 不引入运行时自动 schema create 或应用启动时自动 migration。

## Decisions

1. 角色表使用过滤字段 + `role_id` 的复合索引。

   角色列表和授权回源均以 `role_id` 做稳定排序或返回值，过滤条件集中在 `active` 和 `is_system`。因此在 `Role.Indexes` 中补充 `active, role_id` 和 `is_system, role_id`。备选方案是只保留 `role_id` 唯一索引并依赖数据库排序，但数据增长后会增加过滤后排序成本，不采用。

2. 权限表使用列表过滤字段 + `permission_id` 的复合索引。

   权限列表支持 `module`、`http_method`、`active`、`is_system` 过滤并按 `permission_id` keyset 分页。补充 `active, permission_id`、`module, permission_id`、`http_method, permission_id`，并视实现确认补充 `is_system, permission_id`。保留现有 `http_method, path_template` 唯一索引用于路由身份和 `ListAll` 排序。备选方案是给每个字段仅建单列索引，但无法同时支撑 keyset 排序，不采用。

3. 绑定表补充反向复合索引，而不是拆分已有左前缀。

   `user_roles(user_id, role_id)` 已覆盖按用户查角色、替换和删除绑定；需要补充 `role_id, user_id` 支撑从角色反向 join 用户角色绑定。`role_permissions(role_id, permission_id)` 已覆盖按角色查权限和绑定维护；需要补充 `permission_id, role_id` 支撑从权限反向 join 角色权限绑定。备选方案是分别补单列 `role_id` 或 `permission_id`，但反向复合索引同时提供过滤和关联字段，覆盖能力更好，不采用。

4. 用户昵称 contains 查询使用 PostgreSQL trigram 索引。

   `NicknameContains` 通常对应前置通配符匹配，普通 B-tree `users(nickname)` 对该模式收益有限。应通过 Atlas migration 创建 `pg_trgm` extension 和 `USING gin (nickname gin_trgm_ops)` 索引；Ent schema 可保留或移除普通 `nickname` B-tree，需要以实际 migration diff 和查询用途决定。备选方案是将 contains 改为 prefix 查询以使用 B-tree，但会改变 API 语义，不采用。

5. 所有数据库结构变化通过 Ent 生成和 Atlas migration 交付。

   Ent schema 是结构来源，Atlas SQL migration 是可审查工件。不得手写 `user-service/ent/` 生成代码；涉及 trigram 的 Ent 表达能力如不足，允许在 Atlas migration 中加入 PostgreSQL 专用 SQL，并通过 migration validate 固化。备选方案是运行时执行 `CREATE INDEX`，违反迁移边界，不采用。

## Risks / Trade-offs

- [Risk] 新增索引会增加写入和 migration 执行成本。→ Mitigation: 仅覆盖稳定查询热路径和绑定表反向 FK，不为低频或未使用组合提前建索引。
- [Risk] `pg_trgm` extension 在部分环境未启用或权限不足。→ Mitigation: migration 使用 `CREATE EXTENSION IF NOT EXISTS pg_trgm`，发布前通过 `make user-service-migrate-validate` 和目标环境 migration Job 验证权限。
- [Risk] 过多布尔字段索引选择性不足。→ Mitigation: 布尔字段索引均与 UUID keyset 字段组合，只服务现有列表/授权路径；实现时审查 migration diff，避免重复或等价索引。
- [Risk] Atlas 自动 diff 可能无法表达 trigram GIN 索引。→ Mitigation: 生成 migration 后人工审查 SQL，必要时在 migration 中补充 PostgreSQL 专用索引并再次 validate。
- [Risk] 查询计划是否实际命中索引依赖数据分布。→ Mitigation: 本变更先保证 schema 支撑；后续如有生产慢查询，再基于 `EXPLAIN ANALYZE` 和真实基数微调。

## Migration Plan

1. 更新 Ent schema 索引定义，避免改动业务字段、边和接口。
2. 运行 `make user-service-generate` 更新 Ent 生成代码。
3. 运行 `make user-service-migrate-diff name=optimize-ent-indexes` 生成 Atlas SQL migration。
4. 审查 migration，确认索引名称、重复索引、`pg_trgm` extension 和 GIN trigram 索引正确。
5. 运行 `make user-service-migrate-validate`、相关 Go 测试、`make user-service-architecture-lint`，必要时运行 `make verify`。
6. 发布时按既有流程先由独立 migration Job 或 CI/CD release job 应用 migration，再执行 RBAC seed 和 HTTP rollout。

回滚方式：如新索引导致 migration 执行问题，应通过新的 Atlas migration 删除本次新增索引或回滚发布到未应用 migration 的环境；已经应用到生产的 schema 不应通过手工数据库修改绕过 Atlas 记录。

## Open Questions

无。实施时以最小索引集合和不改变业务语义为准。
