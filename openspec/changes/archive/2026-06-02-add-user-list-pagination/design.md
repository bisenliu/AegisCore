## Context

当前用户服务已有创建用户和按 ID 查询用户接口，路由、controller、service、repository 分层清晰，成功与失败响应统一由 `common/response` 输出。列表类 API 尚无统一分页响应结构，也没有跨服务可复用的分页参数规范化方法，若直接在 `user-services` 内实现会导致后续 list 接口重复处理默认值、offset 计算和响应结构。

本变更横跨 `common` 与 `user-services`：`common` 负责分页响应契约与分页参数公共能力，`user-services` 负责用户列表 API 的 HTTP 参数解析、业务编排和 Ent 查询。

## Goals / Non-Goals

**Goals:**

- 在 `common/response/response.go` 中提供标准分页 payload：`data.items` 和 `data.pagination`。
- 在 `common` 中提供分页参数规范化能力，统一 `page`、`page_size` 默认值和 offset/limit 计算。
- 新增 `GET /api/v1/users` 用户列表接口，支持分页和过滤，并返回与详情接口一致的安全用户 DTO。
- 保持 controller/service/repository 分层：controller 只处理 HTTP query 绑定与响应输出，service 负责过滤参数清洗与编排，repository 负责 Ent predicate、count 和分页查询。
- 补充单元测试覆盖分页默认值、边界值、过滤条件、响应结构和错误映射。

**Non-Goals:**

- 不新增用户修改、删除、禁用等能力。
- 不修改 Ent User schema，不生成 Atlas migration。
- 不引入游标分页、复杂排序、多字段全文检索或跨表查询。
- 不改变现有 `GET /api/v1/users/:id` 和 `POST /api/v1/users` 的响应契约。

## Decisions

### Decision: 在 common 中定义分页响应与分页参数类型

在 `common/response` 中新增 `PaginatedData[T]` 和 `Pagination`，并提供 `NewPaginatedData(items, pagination)` 或等价构造方法，使 list 响应可以直接通过 `response.OK(c, response.NewPaginatedData(users, pagination))` 输出。

原因：分页 payload 是响应契约的一部分，放在 `common/response` 能确保所有服务的 list 响应保持统一结构。

备选方案：在 `user-services/internal/dto` 中定义用户专用分页响应。该方案实现更局部，但会让后续其他 list 接口重复定义 `items/pagination`，不符合共享能力优先放入 `common` 的约束。

### Decision: 分页参数规范化放在 common 公共方法中

在 common 中新增可复用分页 helper，例如 `NormalizePagination(page, pageSize int) PaginationQuery`，输出规范化后的 `Page`、`PageSize`、`Offset`、`Limit`。规则为 `page < 1` 时使用 1，`page_size < 1` 时使用 10。

原因：默认值规则是跨 list 接口可复用行为，不应散落在 controller 或 service 中。

备选方案：依赖 Gin binding 的 default tag 或 DTO `SetDefaults`。该方案只能覆盖未传参数，无法统一处理小于 1 的非法边界，也难以复用 offset 计算。

### Decision: 用户列表过滤字段采用明确白名单

初始支持以下 query 参数：`page`、`page_size`、`name`、`email`、`active`。其中 `name` 使用模糊匹配，`email` 规范化为小写后精确匹配，`active` 使用布尔过滤。返回字段复用 `dto.UserResponse`，不包含 `password`。

原因：这些字段均来自当前 Ent User schema，不需要数据库结构变更；过滤语义简单可测试，能满足常见用户列表查询。

备选方案：支持任意字段过滤或 created_at 时间范围。任意字段过滤会扩大 API 契约和安全风险；时间范围可在后续有明确调用方需求时加入，本次仅保留设计扩展空间。

### Decision: repository 同时返回 items 与 total

`UserRepository` 新增 `List(ctx, input ListUsersInput) ([]*ent.User, int, error)` 或等价结构化返回。repository 基于相同 predicate 构造 count 查询和分页查询，分页查询建议按 `id` 升序保持稳定结果。

原因：`total` 必须反映过滤后的总记录数，只有 repository 最了解 Ent 查询条件，能保证 count 与 items 使用同一组 predicate。

备选方案：service 分别调用 count 和 list 并重复构造过滤条件。该方案会泄漏数据访问细节到 service，破坏分层。

### Decision: 错误处理复用现有响应契约

query 参数绑定失败返回既有 bad request 或 validation failed 响应；数据库非预期错误通过 `response.FromError` 包装为内部错误；列表为空时返回 HTTP 200，`items: []`，`total: 0`，`total_pages: 0`。

原因：列表为空不是错误；错误码和失败信封保持与现有 `api-response-contract` 兼容。

备选方案：列表为空返回 404。该方案不符合列表查询的通用 API 语义，也会增加调用方处理复杂度。

## Risks / Trade-offs

- [Risk] `page_size` 未设置上限可能导致调用方请求过大页面，影响数据库和响应体大小。→ Mitigation：本次规格只定义小于 1 的默认值；实现可在设计基础上增加保守最大值时需同步更新规格。
- [Risk] count 与 items 两次查询之间数据变化会导致极短窗口内总数和列表不完全一致。→ Mitigation：接受读已提交语义，避免为普通列表查询引入事务复杂度。
- [Risk] name 模糊匹配在大表上可能性能不足。→ Mitigation：当前不修改 schema 或索引；后续如有性能问题再提出索引或搜索能力变更。
- [Risk] 泛型分页响应会影响 Swagger 展示。→ Mitigation：在用户服务中可增加用户列表专用文档 DTO 或注解，运行时仍复用 common 分页响应结构。

## Migration Plan

该变更仅新增 API 和 common helper，不需要数据库 migration。部署时随用户服务版本发布即可；回滚时移除新路由后，现有接口保持可用。

## Open Questions

- 是否需要为 `page_size` 增加最大值限制暂不在本次契约中定义，避免超出用户明确需求。
