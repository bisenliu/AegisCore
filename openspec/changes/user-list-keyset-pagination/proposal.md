## Why

当前 `GET /api/v1/users` 使用 page/offset/count 分页，随着用户表增长会带来深页查询性能下降和额外 count 开销。该变更将用户列表原地切换为基于 `user_id` 的 keyset pagination，使列表查询稳定按 `user_id` 前进，并移除不再需要的页码、offset 和总数语义。

## What Changes

- **BREAKING**：`GET /api/v1/users` 不再接受或处理 `page`、`offset`、`count` 相关语义，响应 `data.pagination` 不再包含 `page`、`total`、`total_pages`。
- **BREAKING**：公共分页响应类型 `response.Pagination` 和 `response.PaginatedData` 保持原命名，但其分页语义改为 `page_size`、`next_cursor`、`has_next`。
- 用户列表请求新增或保留 `cursor` query 参数，cursor 表示上一页最后一条记录的 `user_id`；非法 UUID cursor 返回 HTTP 400 统一失败信封。
- 用户列表查询按 `user_id ASC` 排序，并使用 `user_id > cursor` 和 `Limit + 1` 判断 `has_next`，不再执行 `Count` 或 `Offset`。
- 用户列表过滤继续支持 `nickname`、`username`、`status`，并继续排除 `deleted_at IS NOT NULL` 的软删除用户。
- Swagger/OpenAPI 用户列表文档同步删除 page/total 语义，并描述 `cursor`、`page_size`、`next_cursor`、`has_next`。
- Ent schema 和 Atlas migration 增加支持 keyset 查询路径的用户表索引。
- 测试同步删除旧 page/offset/total/total_pages 断言，并覆盖默认/最大 page_size、非法 cursor、响应字段、无 Count/Offset、`user_id > cursor`、`user_id ASC`、`Limit + 1` 产生 `has_next`。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-list-query`：用户列表分页契约从 page/offset/count 改为基于 `user_id` 的 keyset pagination。
- `api-response-contract`：公共分页响应模型保持 `Pagination`、`PaginatedData` 命名，但字段和规范化 helper 改为 keyset 分页语义。
- `api-swagger-documentation`：用户列表接口文档同步描述 cursor/keyset 分页参数和响应字段。
- `database-schema-migrations`：用户表索引策略新增适配 `deleted_at`、`status` 与 `user_id` 的 keyset 查询路径。

## Impact

- 影响代码：`common/contract/response/pagination.go`，用户 feature 的 `api`、`transport/http`、`app`、`infra/postgres`、mapper、Swagger doc，以及相关测试。
- 影响数据库：`user-services/ent/schema` 增加用户表索引，并通过 Atlas 在 `user-services/migrations/` 生成 SQL migration 和更新 `atlas.sum`。
- 影响 API：`GET /api/v1/users` 响应仍使用统一 Envelope，`data.items` 结构不变，但分页请求和响应字段不兼容旧 page/offset/count 语义。
- 影响兼容性：不提供旧参数兼容、不保留旧 page/offset/count 逻辑，客户端必须迁移到 `cursor`、`page_size`、`next_cursor`、`has_next`。
