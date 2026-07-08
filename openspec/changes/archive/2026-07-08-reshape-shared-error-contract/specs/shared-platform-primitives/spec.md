## MODIFIED Requirements

### Requirement: 跨服务契约基础

系统 MUST 在 `common/` 中维护跨服务共享的错误、响应 envelope、分页和 HTTP response helper，以保证服务之间的外部契约保持一致，并保持业务中立。共享错误契约 MUST 使用语义驱动的应用错误模型表达错误类别、业务原因、稳定响应码、公开消息和内部原因，并 MUST NOT 在 `common/contract/errors` 中保存、暴露或推导 HTTP status。HTTP status MUST 只由 `common/http/response` 根据应用错误 `Kind` 推导。

#### Scenario: 返回统一响应

- **WHEN** 服务处理成功响应或错误响应
- **THEN** 系统 MUST 使用共享响应和错误契约表达 code、message、data、pagination 或错误详情

#### Scenario: 新服务复用契约

- **WHEN** 新服务模块需要对外暴露 HTTP API
- **THEN** 该服务 MUST 优先复用 `common/contract/` 和 `common/http/response/` 中的稳定契约，而不是定义不兼容的 envelope

#### Scenario: 契约变更需要规格化

- **WHEN** 共享错误码、响应 envelope 或分页结构需要改变
- **THEN** change MUST 更新相关主规格或 delta spec，并评估所有使用 `common/contract/` 的服务影响

#### Scenario: 新增错误分类

- **WHEN** 需要在 `common/contract/errors` 新增跨服务错误分类或原因
- **THEN** `Kind` 和 `Reason` MUST 保持业务中立或由明确业务边界声明
- **AND** `Kind` MUST 表达低基数错误类别
- **AND** `Reason` MUST 表达稳定、可公开的错误原因
- **AND** HTTP status MUST 由 `common/http/response` 根据 `Kind` 渲染

#### Scenario: 应用错误不暴露 HTTP status

- **WHEN** 调用方创建、包装或检查 `common/contract/errors` 应用错误
- **THEN** 应用错误 MUST 暴露 `Kind`、`Reason`、`Code`、`Message` 和可选 `Cause`
- **AND** 应用错误 MUST NOT 暴露 `HTTPStatus` 字段
- **AND** 应用错误构造 API MUST NOT 接收 HTTP status 参数

#### Scenario: HTTP 层推导错误状态码

- **WHEN** `common/http/response` 写入应用错误响应
- **THEN** 系统 MUST 根据应用错误 `Kind` 推导 HTTP status
- **AND** 请求格式错误或字段校验失败 MUST 渲染为 `400 Bad Request`
- **AND** 未认证或 token 无效、过期、撤销 MUST 渲染为 `401 Unauthorized`
- **AND** 权限不足 MUST 渲染为 `403 Forbidden`
- **AND** 冲突 MUST 渲染为 `409 Conflict`
- **AND** 未找到 MUST 渲染为 `404 Not Found`
- **AND** 服务不可用 MUST 渲染为 `503 Service Unavailable`
- **AND** nil、未知或内部错误 MUST 渲染为 `500 Internal Server Error`

#### Scenario: 应用错误转换和包装

- **WHEN** 系统通过 `FromError` 归一化错误
- **THEN** wrapped application error MUST 保留原始 `Kind`、`Reason`、`Code` 和公开 `Message`
- **AND** nil error MUST 按内部错误处理
- **AND** 未知 error MUST 按内部错误处理并使用非敏感公开 message
- **AND** 原始错误 MUST 只作为内部 `Cause` 保留

#### Scenario: 标准错误链支持

- **WHEN** 调用方使用 `errors.As` 检查 wrapped application error
- **THEN** 系统 MUST 能从错误链中解析出应用错误
- **WHEN** 调用方使用 `errors.Is` 按应用错误类别或原因匹配
- **THEN** 系统 MUST 按 `Kind` 和 `Reason` 的稳定语义进行匹配
- **AND** 内部 `Cause` MUST 继续支持标准 `errors.Is` 和 `errors.As`

#### Scenario: 校验错误响应

- **WHEN** 请求绑定或字段校验失败
- **THEN** `common/validation` 和 `common/http/binding` MUST 生成或传播语义应用错误分类
- **AND** `common/http/response` MUST 将字段校验失败渲染为 `400 Bad Request`
- **AND** 响应 envelope MUST 保持 `success=false`、`code=CodeValidationFailed`、公开 message 和结构化字段错误明细

#### Scenario: 强制改密错误码稳定

- **WHEN** 服务需要表达用户凭据有效但账号要求强制修改密码
- **THEN** 系统 MUST 使用 `CodePasswordChangeRequired`
- **AND** 该 code 的数值 MUST 为 `20006`

#### Scenario: 错误码保持业务中立

- **WHEN** `common/contract/errors` 新增 `CodePasswordChangeRequired`
- **THEN** `common` MUST 只定义共享错误码和通用错误构造能力
- **AND** `common` MUST NOT 承载 user-service 的受限 token 签发、强制改密状态判断或登录响应编排逻辑

#### Scenario: 服务不可用错误

- **WHEN** 服务需要表达临时资源池繁忙、依赖暂时不可用或实例无法处理当前请求
- **THEN** 共享错误契约 MUST 提供业务中立的服务不可用 `Kind`
- **AND** `common/http/response` MUST 将该 `Kind` 渲染为 `503 Service Unavailable`
- **AND** 具体业务边界 MUST 提供不泄露内部实现细节的公开消息

#### Scenario: 不保留旧兼容路径

- **WHEN** 系统完成语义应用错误模型迁移
- **THEN** `common/contract/errors` MUST NOT 保留旧 `HTTPStatus` 字段
- **AND** 系统 MUST NOT 保留接收 HTTP status 的旧 factory API
- **AND** 系统 MUST NOT 保留从旧状态码直连模型到新模型的兼容适配层
