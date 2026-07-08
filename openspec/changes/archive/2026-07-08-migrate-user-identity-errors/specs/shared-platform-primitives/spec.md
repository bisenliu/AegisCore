## ADDED Requirements

### Requirement: 服务内 shared identity 应用错误契约

系统 MUST 允许服务内 shared kernel 为稳定身份错误定义应用错误，使这些错误同时携带 `Kind`、`Reason`、`Code`、`Message` 并可被共享 response helper 直接渲染。该契约 MUST 保持服务内业务归属清晰，且不得把 user-service 的用户错误映射表上移到 `common` 或跨 feature 全局包。

#### Scenario: shared identity 定义可渲染应用错误

- **WHEN** `user-service/internal/shared/identity` 定义用户不存在或用户已存在错误
- **THEN** 错误 MUST 携带共享错误契约所需的 `Kind`、`Reason`、`Code` 和公开 `Message`
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误

#### Scenario: shared identity 保持业务判断语义

- **WHEN** 调用方通过 `errors.Is` 判断 `identity.ErrUserNotFound` 或 `identity.ErrUserAlreadyExists`
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配

#### Scenario: common 不承载用户错误映射

- **WHEN** 用户身份错误迁移为应用错误
- **THEN** `common/` MUST 只提供业务中立的错误契约、应用错误构造和 response 渲染 helper
- **AND** 系统 MUST NOT 在 `common` 或用户 feature 外新增用户错误到 HTTP 响应的全局映射表
