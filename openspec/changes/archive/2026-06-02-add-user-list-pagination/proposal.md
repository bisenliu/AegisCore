## Why

当前用户服务只支持按 ID 查询单个用户，缺少面向管理后台或调用方批量展示用户资料的列表接口；同时通用响应契约尚未提供标准分页数据结构，后续 list 类接口容易重复实现分页解析和响应包装。

本变更将用户列表分页查询沉淀为可复用模式，在保持现有响应信封兼容的前提下扩展 `common/response`，并为用户服务新增可过滤的分页列表 API。

## What Changes

- 在 `common/response` 中新增标准分页数据结构，成功响应仍使用 `success/code/message/data` 信封，分页列表数据位于 `data.items` 和 `data.pagination`。
- 在 `common` 中新增分页查询公共方法，用于解析或规范化 `page`、`page_size`，当 `page` 未传或小于 1 时默认 1，当 `page_size` 未传或小于 1 时默认 10。
- 在 `user-services` 新增用户列表接口，支持分页查询并返回不含 `password` 的用户资料列表。
- 用户列表接口支持过滤字段设计：`name` 模糊匹配、`email` 精确匹配、`active` 布尔过滤，并保留按创建时间范围过滤的扩展空间。
- 保持现有 `GET /api/v1/users/:id`、创建用户接口、错误码和失败响应结构兼容。

## Capabilities

### New Capabilities
- `user-list-query`: 定义用户分页列表查询 API、过滤参数、返回结构、分层行为和错误处理。

### Modified Capabilities
- `api-response-contract`: 扩展成功响应契约，定义可复用分页 payload 和分页参数规范化行为。

## Impact

- 影响代码：`common/response/response.go`，以及可能新增的 common 分页 helper；`user-services/internal/router/router.go`、`user-services/internal/controller/user_controller.go`、`user-services/internal/service/user_service.go`、`user-services/internal/repository/user_repository.go`、用户 DTO 和相关测试。
- API 影响：新增 `GET /api/v1/users` 列表接口；成功响应仍使用 HTTP 200 和业务码 `0`，失败响应继续使用既有统一错误码。
- 兼容性：不改变现有响应信封顶层字段，不改变现有用户详情接口路径与响应字段；新增分页响应仅用于 list 类接口。
- 数据模型：不要求修改 Ent User schema 或生成数据库 migration。
- 文档影响：需要在规格和后续实现中补充 Swagger/OpenAPI 对分页响应、查询参数和过滤字段的说明。
