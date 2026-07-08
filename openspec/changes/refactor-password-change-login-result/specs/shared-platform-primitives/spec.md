## MODIFIED Requirements

### Requirement: 跨服务契约基础

系统 MUST 在 `common/` 中维护跨服务共享的错误、响应 envelope、分页和 HTTP response helper，以保证服务之间的外部契约保持一致。共享错误契约 MUST 能表达临时服务不可用状态，供服务在资源池耗尽或依赖临时不可用时返回稳定 HTTP 响应。共享响应码契约 MUST 能表达认证流程中的稳定业务状态，包括凭据有效但必须先完成密码修改。

#### Scenario: 返回统一响应

- **WHEN** 服务处理成功响应或错误响应
- **THEN** 系统 MUST 使用共享响应和错误契约表达 code、message、data、pagination 或错误详情

#### Scenario: 新服务复用契约

- **WHEN** 新服务模块需要对外暴露 HTTP API
- **THEN** 该服务 MUST 优先复用 `common/contract/` 和 `common/http/response/` 中的稳定契约，而不是定义不兼容的 envelope

#### Scenario: 契约变更需要规格化

- **WHEN** 共享错误码、响应 envelope 或分页结构需要改变
- **THEN** change MUST 更新相关主规格或 delta spec，并评估所有使用 `common/contract/` 的服务影响

#### Scenario: 新增认证流程业务码

- **WHEN** 需要在 `common/contract/errors` 表达凭据有效但必须先完成密码修改
- **THEN** 共享错误码契约 MUST 提供 `CodePasswordChangeRequired`
- **AND** `CodePasswordChangeRequired` MUST 使用认证码段数值 `20006`
- **AND** 该 code MUST 可被响应 envelope 使用，以便调用方稳定识别强制改密流程

#### Scenario: 强制改密登录不新增通用错误构造函数

- **WHEN** user-service 登录凭据有效但账号要求强制修改密码
- **THEN** auth HTTP transport MUST 可以使用 `CodePasswordChangeRequired` 构造专用 envelope
- **AND** 共享错误契约 MUST NOT 要求提供 `ReasonPasswordChangeRequired`
- **AND** 共享错误契约 MUST NOT 要求提供 `PasswordChangeRequiredError` 或等价通用 error factory

#### Scenario: 错误码保持边界清晰

- **WHEN** `common/contract/errors` 维护共享错误码和 reason
- **THEN** `common` MUST 只定义稳定 code 常量和通用错误构造能力
- **AND** `common` MUST NOT 承载 user-service 的受限 token 签发、强制改密状态判断或登录响应编排

#### Scenario: 服务不可用错误

- **WHEN** 服务需要表达临时资源池繁忙、依赖暂时不可用或实例无法处理当前请求
- **THEN** 共享错误契约 MUST 提供业务中立的服务不可用错误分类
- **AND** 该错误 MUST 渲染为 `503 Service Unavailable`
- **AND** 具体业务边界 MUST 提供不泄露内部实现细节的公开消息
