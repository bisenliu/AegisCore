## ADDED Requirements

### Requirement: 认证领域状态判断测试覆盖

`auth-session-management` 的 auth domain 测试 MUST 直接覆盖 `UserCredential` 对用户状态的领域判断。测试 MUST 固定普通登录、强制改密登录和受限改密流程当前使用的状态语义，并 MUST NOT 通过旧状态别名、旧 token 类型复用、旧错误码、旧字段或兼容 helper 表达预期。

#### Scenario: 普通状态允许普通登录

- **WHEN** `UserCredential.Status` 为 `identity.UserStatusNormal`
- **THEN** `CanLogin` MUST 返回 `true`
- **AND** `RequiresPasswordChange` MUST 返回 `false`
- **AND** `CanChangePassword` MUST 返回 `false`

#### Scenario: 强制改密状态只允许受限改密流程

- **WHEN** `UserCredential.Status` 为 `identity.UserStatusMustChangePassword`
- **THEN** `CanLogin` MUST 返回 `false`
- **AND** `RequiresPasswordChange` MUST 返回 `true`
- **AND** `CanChangePassword` MUST 返回 `true`

#### Scenario: 不可登录状态拒绝认证流程

- **WHEN** `UserCredential.Status` 为 `identity.UserStatusDisabled` 或未知状态值
- **THEN** `CanLogin` MUST 返回 `false`
- **AND** `RequiresPasswordChange` MUST 返回 `false`
- **AND** `CanChangePassword` MUST 返回 `false`

#### Scenario: auth domain 测试使用语义化断言

- **WHEN** auth domain 测试覆盖 `UserCredential` 状态判断
- **THEN** 测试 MUST 使用 `testify/require` 或等价语义化断言表达布尔和值预期
- **AND** 测试 MUST NOT 使用机械 `Fail` / `Failf` 替换或旧手写断言兼容 helper
