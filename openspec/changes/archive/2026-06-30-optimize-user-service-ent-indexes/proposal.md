## Why

当前 user-service 的角色、权限和 RBAC 绑定表查询已经形成稳定的分页、授权回源、有效权限聚合和 policy loader 访问模式，但部分 Ent schema 仍缺少与这些访问路径匹配的复合索引或反向外键索引。随着用户、角色、权限和绑定数据增长，这会放大列表查询、授权缓存 miss 回源、权限聚合和策略加载的数据库扫描成本。

## What Changes

- 为 `Role`、`Permission`、`UserRole` 和 `RolePermission` Ent schema 补充与现有查询模式匹配的索引。
- 为 `User.nickname` 的 contains 查询明确 PostgreSQL trigram 索引策略，避免继续依赖对前置通配符收益有限的普通 B-tree 索引。
- 生成 Ent 代码和 Atlas SQL migration，确保数据库 schema 变更以可审查 migration 工件交付。
- 增加或调整相关验证，覆盖 Ent 生成、migration 校验、架构 lint 和相关 store/query 测试。
- 不改变 HTTP API、OpenAPI、业务错误语义、权限判定结果、RBAC seed 数据或用户状态规则。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-identity-management`: 明确用户资料列表和昵称 contains 查询在数据增长时必须具备可审查的数据库索引支撑。
- `rbac-access-control`: 明确角色、权限、用户角色绑定、角色权限绑定、有效权限查询和 Casbin policy loader 的持久化查询必须具备与访问路径匹配的索引支撑。

## Impact

- 影响 `user-service/ent/schema/` 中 `Role`、`Permission`、`UserRole`、`RolePermission` 和可能的 `User` schema 索引定义。
- 影响 `user-service/ent/` 生成代码和 `user-service/migrations/` Atlas SQL migration。
- 影响 PostgreSQL 索引数量和写入成本；需要控制索引数量，优先覆盖热路径和已存在的稳定查询。
- 不影响 REST API、OpenAPI 文档、Gin 路由、Casbin policy 语义、JWT/session 行为、部署镜像边界或 `common` 模块。
