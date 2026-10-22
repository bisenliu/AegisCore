## ADDED Requirements

### Requirement: 跨服务 UUID 使用 Go 标准库

`common/runtime/id` 对外提供的 UUID 值 MUST 使用 Go 标准库 `uuid.UUID`，默认生成 MUST 保持 UUID v7；现有返回 error 的导出函数签名 MUST 保持可调用。项目一方 Go 代码 MUST 使用标准库 `uuid` 表达和解析 UUID，不得通过第三方 UUID 类型维护并行 ID 契约。

#### Scenario: 默认生成 ID

- **WHEN** 调用 `common/runtime/id` 的 UUID 或 UUID 字符串生成函数
- **THEN** 返回值 MUST 是有效 UUID v7，字符串 MUST 保持小写、带连字符的规范格式

#### Scenario: UUID 在服务边界往返

- **WHEN** 用户、角色、权限或会话 ID 经解析、JSON/JWT 编码、Redis 存储或 Ent 持久化后再次读取
- **THEN** UUID 值与规范字符串 MUST 保持一致，非法 UUID 输入 MUST 按现有错误边界拒绝
- **AND** 现有需要 UUID v4 的调用 MUST 继续生成 v4
