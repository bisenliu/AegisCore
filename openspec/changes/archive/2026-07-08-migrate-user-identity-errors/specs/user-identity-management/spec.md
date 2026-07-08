## ADDED Requirements

### Requirement: 用户身份错误应用错误渲染

系统 MUST 将用户资料能力中的用户已存在和用户不存在错误表达为可由共享 response helper 直接渲染的应用错误，并保持用户 HTTP 边界无专用 sentinel-to-HTTP 兼容映射。

#### Scenario: 用户已存在渲染为冲突响应

- **WHEN** 用户创建流程返回 `identity.ErrUserAlreadyExists`
- **THEN** 用户 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前用户已存在公开文案

#### Scenario: 用户不存在渲染为未找到响应

- **WHEN** 用户详情查询流程返回 `identity.ErrUserNotFound`
- **THEN** 用户 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前用户不存在公开文案

#### Scenario: 用户业务错误保留 errors.Is 语义

- **WHEN** 用户 feature 或测试需要判断用户已存在或用户不存在错误
- **THEN** `errors.Is(err, identity.ErrUserAlreadyExists)` 与 `errors.Is(err, identity.ErrUserNotFound)` MUST 继续返回正确结果
- **AND** 系统 MUST NOT 为用户 HTTP transport 保留 `toUserHTTPError` 或等价兼容函数

### Requirement: 用户 HTTP controller 统一错误出口

用户 HTTP controller MUST 对业务 service 返回错误使用共享 `response.Fail` 入口，避免在 transport 层重复维护用户身份错误到 HTTP 响应的映射。

#### Scenario: 用户创建业务错误

- **WHEN** `CreateUser` controller 调用用户创建 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用用户专用错误 mapper

#### Scenario: 用户详情查询业务错误

- **WHEN** `GetByUserID` controller 调用用户查询 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用用户专用错误 mapper

#### Scenario: 用户列表查询业务错误

- **WHEN** `ListUsers` controller 调用用户列表 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用用户专用错误 mapper
