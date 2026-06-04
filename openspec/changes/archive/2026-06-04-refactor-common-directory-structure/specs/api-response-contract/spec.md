## ADDED Requirements

### Requirement: Host response contract under common contract path
API 响应契约代码 SHALL 位于 `common/contract/response`，以表达响应信封、业务错误码、失败响应 helper 和分页模型属于跨服务 API contract。目录迁移 MUST 保持 Go package name、导出 API 语义、HTTP status、业务错误码、JSON 字段和对外 message 行为不变。

#### Scenario: Response package path changes without envelope changes
- **WHEN** controller 或 middleware 使用迁移后的 response 包构造成功或失败响应
- **THEN** 响应 JSON MUST 继续使用 `success`、`code`、`message`、`data` 和 `errors` 字段约定
- **THEN** 成功响应、参数错误、认证失败、冲突、未找到和内部错误的业务码 MUST 保持不变

#### Scenario: Pagination helpers remain contract-owned
- **WHEN** list 类接口使用分页响应 helper
- **THEN** 分页模型 MUST 继续位于响应契约包边界内
- **THEN** `data.items` 和 `data.pagination` 的 JSON 结构 MUST 保持不变

#### Scenario: Response imports are synchronized
- **WHEN** 响应契约包迁移到 `common/contract/response`
- **THEN** 仓库内 Go imports、测试、Swagger-adjacent 引用、文档和 OpenSpec 路径引用 MUST 同步更新
- **THEN** 实现 MUST NOT 保留与新路径行为分叉的旧响应包
