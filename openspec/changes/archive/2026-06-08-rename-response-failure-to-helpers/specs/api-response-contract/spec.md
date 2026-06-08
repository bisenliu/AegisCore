## MODIFIED Requirements

### Requirement: Organize response package by contract responsibilities
系统 SHALL 将 `common/contract/response` 中的响应信封、分页模型/计算、标准消息常量和响应 helper 组织到职责明确的源文件中。语义响应 helper 文件 SHALL 使用能反映 convenience utilities 职责的命名，例如 `helpers.go`，而不得使用会让维护者误解为仅包含失败模型或失败信封的文件名。该组织变更 MUST 保持 `common/contract/response` 包名、导出 API、HTTP status、业务错误码、JSON 字段和对外消息语义不变。

#### Scenario: Response envelope behavior is preserved after file organization changes
- **Given** controller 调用 `response.OK`、`response.Created`、`response.Fail` 或标准语义响应 helper
- **When** `common/contract/response` 文件组织被调整
- **Then** 响应 JSON MUST 继续使用 `success`、`code`、`message`、`data` 和 `errors` 字段约定
- **Then** 成功响应、失败响应、校验失败响应和 token 失败响应的 HTTP status 与业务码 MUST 与调整前一致
- **Then** 调用方 MUST NOT 因文件组织调整改用新的 Go package import 路径

#### Scenario: Pagination helpers remain reusable
- **Given** list 类接口调用 `response.NormalizePagination`、`response.NewPagination` 或 `response.NewPaginatedData`
- **When** 分页类型和计算逻辑位于聚焦文件中
- **Then** 默认 `page=1` 和 `page_size=10` 的行为 MUST 保持不变
- **Then** `data.items` 和 `data.pagination` 的 JSON 结构 MUST 保持不变
- **Then** nil items MUST 继续序列化为空数组语义

#### Scenario: Response constants remain centralized
- **Given** 响应 helper 构造成功、认证失败或内部错误消息
- **When** 标准消息常量位于聚焦文件中
- **Then** `ok`、`created`、`internal server error` 和通用认证失败消息值 MUST 保持不变
- **Then** 标准业务码常量 MUST 继续位于 `common/contract/response` 包中供调用方复用

#### Scenario: Semantic response helpers use descriptive file naming
- **Given** 维护者需要定位 `BadRequest`、`ValidationFailed`、`Unauthenticated`、`Forbidden`、`Conflict` 或 `NotFound` 等语义响应便利函数
- **When** 维护者浏览 `common/contract/response` 源文件
- **Then** 这些 helper MUST 位于名称能表达响应 helper 职责的文件中
- **Then** 文件名 MUST NOT 暗示该文件只包含失败模型、失败信封或错误类型定义
- **Then** helper 所属 Go package MUST 继续为 `response`
