## ADDED Requirements

### Requirement: 认证状态错误码

共享错误契约 MUST 提供 `CodePasswordChangeRequired = 20006`，用于表达凭据校验通过但调用方必须先完成密码修改的认证状态。该错误码 MUST 位于 `common/contract/errors`，并 MUST 能被统一响应 envelope 和 OpenAPI 错误码枚举稳定渲染。

#### Scenario: 强制改密错误码稳定

- **WHEN** 服务需要表达用户凭据有效但账号要求强制修改密码
- **THEN** 系统 MUST 使用 `CodePasswordChangeRequired`
- **AND** 该 code 的数值 MUST 为 `20006`

#### Scenario: 错误码保持业务中立

- **WHEN** `common/contract/errors` 新增 `CodePasswordChangeRequired`
- **THEN** `common` MUST 只定义共享错误码和通用错误构造能力
- **AND** `common` MUST NOT 承载 user-service 的受限 token 签发、强制改密状态判断或登录响应编排逻辑
