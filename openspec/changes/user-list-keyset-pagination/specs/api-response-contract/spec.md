## MODIFIED Requirements

### Requirement: Return paginated success responses

系统 MUST 在 `common/contract/response` 中提供可复用分页响应数据结构。分页列表成功响应 MUST 保持统一 `Envelope` 顶层字段，并将列表数据包装到 `data.items`，分页元信息包装到 `data.pagination`。公共响应类型 MUST 继续命名为 `Pagination` 和 `PaginatedData`，但分页元信息 MUST 使用 keyset pagination 语义。

#### Scenario: Return paginated list payload
- **Given** controller 成功处理分页列表请求并获得列表数据和分页元信息
- **When** controller 使用 `response.OK` 返回分页数据
- **Then** 系统返回 HTTP 200
- **Then** 响应 JSON 包含 `success: true`、`code: 0`、`message: ok`
- **Then** `data.items` MUST 包含当前页业务数据数组
- **Then** `data.pagination` MUST 只包含 `page_size`、`next_cursor`、`has_next`
- **Then** `data.pagination` MUST NOT 包含 `page`、`offset`、`total` 或 `total_pages`

#### Scenario: Return empty paginated list
- **Given** 分页列表请求没有匹配到任何记录
- **When** controller 返回分页数据
- **Then** 系统返回 HTTP 200
- **Then** `data.items` MUST 为空数组
- **Then** `data.pagination.page_size` MUST 为规范化后的 page size
- **Then** `data.pagination.next_cursor` MUST 为空或省略
- **Then** `data.pagination.has_next` MUST 为 `false`

#### Scenario: Preserve pagination type names
- **Given** 调用方引用公共分页响应类型
- **When** 代码构造分页列表响应
- **Then** 系统 MUST 继续提供 `response.Pagination`
- **Then** 系统 MUST 继续提供 `response.PaginatedData`
- **Then** 系统 MUST NOT 新增 `CursorPagination` 或 `CursorPaginatedData` 作为公共响应类型

### Requirement: Normalize pagination query parameters

系统 MUST 在 common 中提供可复用 page size 规范化方法，用于为 list 类接口统一处理 `page_size` 默认值和最大值。系统 MUST NOT 在公共分页契约中继续提供 page/offset/count 计算语义。

#### Scenario: Default missing page size parameter
- **Given** list 请求未提供 `page_size`
- **When** 系统规范化 page size
- **Then** `page_size` MUST 使用默认值 `10`
- **Then** 数据库查询 limit MUST 为 `10`

#### Scenario: Default invalid page size parameter
- **Given** list 请求提供的 `page_size` 小于 `1`
- **When** 系统规范化 page size
- **Then** 小于 `1` 的 `page_size` MUST 使用默认值 `10`

#### Scenario: Cap page size parameter
- **Given** list 请求提供的 `page_size` 大于 `100`
- **When** 系统规范化 page size
- **Then** 大于 `100` 的 `page_size` MUST 使用最大值 `100`

#### Scenario: Remove page and offset helpers
- **Given** 仓库内代码使用公共分页契约
- **When** 实现 keyset pagination 响应契约
- **Then** `common/contract/response` MUST NOT 继续提供 `PaginationQuery`
- **Then** `common/contract/response` MUST NOT 继续提供 `NormalizePagination(page, pageSize)`
- **Then** `response.Pagination` MUST NOT 包含 `Page`、`Offset`、`Total` 或 `TotalPages` 字段
