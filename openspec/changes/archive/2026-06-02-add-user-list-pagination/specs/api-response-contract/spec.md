## ADDED Requirements

### Requirement: Return paginated success responses

系统 MUST 在 `common/response` 中提供可复用分页响应数据结构。分页列表成功响应 MUST 保持统一 `Envelope` 顶层字段，并将列表数据包装到 `data.items`，分页元信息包装到 `data.pagination`。

#### Scenario: Return paginated list payload
- **Given** controller 成功处理分页列表请求并获得列表数据和分页元信息
- **When** controller 使用 `response.OK` 返回分页数据
- **Then** 系统返回 HTTP 200
- **Then** 响应 JSON 包含 `success: true`、`code: 0`、`message: ok`
- **Then** `data.items` MUST 包含当前页业务数据数组
- **Then** `data.pagination` MUST 包含 `page`、`page_size`、`total`、`total_pages`

#### Scenario: Return empty paginated list
- **Given** 分页列表请求没有匹配到任何记录
- **When** controller 返回分页数据
- **Then** 系统返回 HTTP 200
- **Then** `data.items` MUST 为空数组
- **Then** `data.pagination.total` MUST 为 `0`
- **Then** `data.pagination.total_pages` MUST 为 `0`

### Requirement: Normalize pagination query parameters

系统 MUST 在 common 中提供可复用分页参数规范化方法，用于为 list 类接口统一处理 `page`、`page_size` 默认值并计算数据库分页参数。

#### Scenario: Default missing pagination parameters
- **Given** list 请求未提供 `page` 或 `page_size`
- **When** 系统规范化分页参数
- **Then** `page` MUST 使用默认值 `1`
- **Then** `page_size` MUST 使用默认值 `10`
- **Then** 数据库查询 offset MUST 为 `0`
- **Then** 数据库查询 limit MUST 为 `10`

#### Scenario: Default invalid pagination parameters
- **Given** list 请求提供的 `page` 小于 `1` 或 `page_size` 小于 `1`
- **When** 系统规范化分页参数
- **Then** 小于 `1` 的 `page` MUST 使用默认值 `1`
- **Then** 小于 `1` 的 `page_size` MUST 使用默认值 `10`

#### Scenario: Calculate total pages
- **Given** `total` 为 `128` 且 `page_size` 为 `20`
- **When** 系统构造分页元信息
- **Then** `total_pages` MUST 为 `7`
