## Context

`GET /api/v1/users` 当前通过 `page`、`page_size`、offset 和 count 查询用户列表，并在 `common/contract/response.Pagination` 中暴露 `page`、`total`、`total_pages`。该方式对深页查询不友好，且每次列表请求都需要额外 count。用户表已经具备外部 UUID `user_id`，可以作为稳定前进游标，将列表分页原地切换为基于 `user_id` 的 keyset pagination。

变更横跨 `common` 响应契约、用户 feature 的 HTTP/app/infra 分层、Swagger 文档、Ent schema、Atlas migration 和测试。实现必须保持 `response.Pagination`、`response.PaginatedData` 类型名不变，不新增 `CursorPagination` 或 `CursorPaginatedData`。

## Goals / Non-Goals

**Goals:**

- 将 `GET /api/v1/users` 原地改为 `cursor` + `page_size` 请求协议和 `page_size` + `next_cursor` + `has_next` 响应协议。
- 删除旧 page/offset/count 逻辑，包括公共 `PaginationQuery`、`NormalizePagination(page, pageSize)`、`Total`、`TotalPages`、`Page`、`Offset`。
- 保持 controller/service/repository 分层：controller 解析 HTTP query 和 cursor UUID，service 编排 store 返回结果，infra/postgres 封装 Ent predicate、排序、limit 和 hasNext 判断。
- 使用 `user_id ASC`、`user_id > cursor`、`Limit + 1` 实现 keyset pagination，并避免 Count 和 Offset。
- 通过 Ent schema 和 Atlas migration 增加适配列表查询路径的索引。
- 同步更新 Swagger 和测试，确保公开 API 文档与运行时行为一致。

**Non-Goals:**

- 不兼容旧 `page`、`offset`、`count` 语义，不为旧客户端提供双协议过渡。
- 不新增新的公共分页类型名，也不引入通用 cursor 编码框架。
- 不改变用户资料 item 字段、认证要求、响应 Envelope 顶层结构或用户过滤字段集合。
- 不手写 `user-services/ent/` 生成代码。

## Decisions

1. 公共分页类型保留原命名但替换字段语义。

   `common/contract/response.Pagination` 只保留 `PageSize`、`NextCursor`、`HasNext`，`PaginatedData[T]` 继续包装 `items` 和 `pagination`。这样保持跨服务分页契约类型入口稳定，同时明确本仓库当前分页协议已切换到 keyset。备选方案是新增 `CursorPagination`，但用户明确要求不得新增 cursor 前缀类型，且双类型会制造兼容期和选择成本。

2. Cursor 使用明文 `user_id` UUID 字符串。

   `cursor` query 参数和 `next_cursor` 响应字段直接承载上一页最后一条记录的 `user_id`。controller 中新增 `ParseListCursor`，空 cursor 表示首页，非法 UUID 返回 HTTP 400。备选方案是使用 opaque/base64 cursor，但当前需求明确基于 `user_id`，直接 UUID 便于测试和排障。

3. App 层以领域输入表达 keyset，infra 层拥有 Ent 细节。

   `ListUsersQuery` 包含 `Cursor *uuid.UUID`、`PageSize`、`Limit` 和过滤字段；service 转为 `ListUsersInput{AfterUserID: query.Cursor, Limit: query.Limit, ...}`。`UserProfileStore.ListUsers` 返回 `([]userdomain.User, bool, error)`，bool 表示 `hasNext`。Ent predicate、`UserIDGT`、`Order(entuser.ByUserID())`、`Limit(input.Limit + 1)` 和切片逻辑都位于 `infra/postgres`。

4. `next_cursor` 只在存在下一页且当前页有 item 时生成。

   service 在 `hasNext && len(users) > 0` 时取 `users[len(users)-1].UserID.String()`。这避免空结果返回无意义 cursor，并使客户端只在 `has_next=true` 时继续请求。

5. 索引按未软删除和 status 高频过滤路径补强。

   Ent schema 增加 `index.Fields("deleted_at", "user_id")` 支持默认未删除 keyset 查询；如果 status 高频过滤，则增加 `index.Fields("status", "deleted_at", "user_id")` 支持 status + 未删除 + user_id 前进。迁移通过 Atlas 生成到 `user-services/migrations/` 并更新 `atlas.sum`，实现不通过运行时自动建表修改 schema。

## Risks / Trade-offs

- [Risk] 旧客户端仍发送 `page` 时不再获得页码语义 → Mitigation：Swagger、规格和测试明确删除 page/total 语义，发布说明标记 breaking change。
- [Risk] UUID v4 的 `user_id ASC` 不是创建时间顺序 → Mitigation：本次需求明确基于 `user_id` keyset，规格只承诺按 `user_id ASC` 稳定排序，不承诺按创建时间排序。
- [Risk] `user_id > cursor` 在游标记录被删除后仍可继续前进，但客户端可能跳过已删除记录 → Mitigation：列表默认只返回未软删除用户，keyset 语义基于排序键位置，不依赖 cursor 对应记录仍存在。
- [Risk] 索引变更需要迁移审查和部署顺序管理 → Mitigation：通过 Ent schema + Atlas 生成 SQL，人工审查索引语句并校验 `atlas.sum`，部署前应用 migration。
- [Risk] 公共响应契约变化会影响其他 list 使用方 → Mitigation：仓库内同步搜索并更新所有 `NormalizePagination`、旧 `Pagination` 字段和旧分页断言，避免半迁移状态。

## Migration Plan

1. 修改 `common/contract/response/pagination.go`，删除旧 page/offset/count helper 和字段，新增 `NormalizePageSize`、新 `NewPagination`、保留 `NewPaginatedData` nil items 行为。
2. 修改用户列表 API DTO、validation、controller、mapper、app service/ports/input/result，以及 `infra/postgres` 查询实现。
3. 修改 Ent `User` schema 索引，运行 `go generate ./ent` 刷新生成代码。
4. 在 `user-services/` 运行 `./scripts/migrate-diff.sh <name>` 生成索引 migration，审查 SQL 后校验或更新 `atlas.sum`。
5. 更新 Swagger 注解和生成文档，删除旧 page/total 描述，增加 cursor/keyset 字段描述。
6. 更新单元测试和集成测试，删除旧断言并新增 keyset 断言。
7. 运行 `go test ./...` 于 `common/` 和 `user-services/`。

Rollback 策略：该变更是 API breaking change，代码回滚需要同时回滚响应契约、用户列表实现、Swagger 文档和测试。数据库新增索引可保留，不影响旧查询语义；如必须回滚 schema，应通过新的 Atlas migration 删除新增索引，不能手改历史 migration。

## Open Questions

- 无待决问题。`status` 高频过滤索引按需求纳入实现和 migration 审查。
