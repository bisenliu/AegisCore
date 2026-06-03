## ADDED Requirements

### Requirement: Organize response package by contract responsibilities
系统 SHALL 将 `common/response` 中的响应信封、分页模型/计算、标准消息常量和失败响应 helper 组织到职责明确的源文件中。该组织变更 MUST 保持 `common/response` 包名、导出 API、HTTP status、业务错误码、JSON 字段和对外消息语义不变。

#### Scenario: Response envelope behavior is preserved after file split
- **Given** controller 调用 `response.OK`、`response.Created`、`response.Fail` 或标准失败 helper
- **When** `common/response` 文件组织被拆分
- **Then** 响应 JSON MUST 继续使用 `success`、`code`、`message`、`data` 和 `errors` 字段约定
- **Then** 成功响应、失败响应、校验失败响应和 token 失败响应的 HTTP status 与业务码 MUST 与拆分前一致
- **Then** 调用方 MUST NOT 因文件拆分改用新的 Go package import 路径

#### Scenario: Pagination helpers remain reusable
- **Given** list 类接口调用 `response.NormalizePagination`、`response.NewPagination` 或 `response.NewPaginatedData`
- **When** 分页类型和计算逻辑被移动到聚焦文件
- **Then** 默认 `page=1` 和 `page_size=10` 的行为 MUST 保持不变
- **Then** `data.items` 和 `data.pagination` 的 JSON 结构 MUST 保持不变
- **Then** nil items MUST 继续序列化为空数组语义

#### Scenario: Response constants remain centralized
- **Given** 响应 helper 构造成功、认证失败或内部错误消息
- **When** 标准消息常量被移动到聚焦文件
- **Then** `ok`、`created`、`internal server error` 和通用认证失败消息值 MUST 保持不变
- **Then** 标准业务码常量 MUST 继续位于 `common/response` 包中供调用方复用
