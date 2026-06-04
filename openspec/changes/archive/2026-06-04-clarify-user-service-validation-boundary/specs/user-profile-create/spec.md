## ADDED Requirements

### Requirement: Validate and normalize create input before service business flow
系统 MUST 在用户创建 Service 执行业务编排前完成创建请求的请求级清洗和基础校验。`nickname`、`username` 和 `password` 的空白裁剪、必填校验、长度/格式校验和状态枚举校验 MUST 位于 Controller、共享请求校验器或服务内 Validation 层，而不是作为用户创建 Service 的主要职责。

#### Scenario: Trim create user fields before service call
- **Given** 调用方提交 `nickname`、`username` 或 `password` 前后包含空白的创建用户请求
- **When** controller 处理创建用户请求并调用 Service
- **Then** 空白裁剪 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Service MUST 使用已规范化的 `nickname`、`username` 和 `password` 执行业务流程

#### Scenario: Reject blank create fields before persistence checks
- **Given** 创建用户请求的 `nickname`、`username` 或 `password` 在裁剪后为空
- **When** controller 处理创建用户请求
- **Then** 请求 MUST 在进入用户名唯一性检查和持久化创建前被判定为校验失败
- **Then** 系统 MUST 返回统一 HTTP 400 失败响应信封
- **Then** Service MUST NOT 将空值基础校验作为创建用户的主要业务分支

#### Scenario: Keep uniqueness and password hashing in service
- **Given** 创建用户请求已经完成请求级校验和规范化
- **When** Service 创建用户
- **Then** Service MUST 检查用户名唯一性
- **Then** Service MUST 将明文密码转换为 `password_hash`
- **Then** Service MUST 将用户已存在领域错误映射为统一冲突响应错误
