## ADDED Requirements

### Requirement: Feature-local domain 应用错误契约

系统 MUST 允许 user-service feature-local domain 为稳定业务错误定义应用错误，使这些错误同时携带 `Kind`、`Reason`、`Code`、`Message` 并可被共享 response helper 直接渲染。该契约 MUST 保持业务归属清晰，且不得把 user-service 的权限错误映射表上移到 `common`、`internal/shared` 或跨 feature 全局包。

#### Scenario: permission domain 定义可渲染应用错误

- **WHEN** `user-service/internal/features/permission/domain` 定义权限已存在、权限不存在、权限输入无效或系统权限保护错误
- **THEN** 错误 MUST 携带共享错误契约所需的 `Kind`、稳定 `Reason`、`Code` 和中文公开 `Message`
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误

#### Scenario: feature-local domain 保持业务判断语义

- **WHEN** 调用方通过 `errors.Is` 判断 permission domain 导出的稳定业务错误
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

#### Scenario: common 保持业务中立

- **WHEN** 权限目录错误迁移为应用错误
- **THEN** `common/` MUST 只提供业务中立的错误契约、应用错误构造和 response 渲染 helper
- **AND** 系统 MUST NOT 在 `common` 新增权限目录专用 `Reason` 常量、公开消息、错误变量或权限错误到 HTTP 响应的全局映射表

#### Scenario: 不新增跨模块权限错误注册表

- **WHEN** feature-local domain 应用错误需要被 HTTP controller 渲染
- **THEN** controller MUST 通过共享 `response.Fail` 和错误自身携带的契约信息完成渲染
- **AND** 系统 MUST NOT 新增跨模块权限错误映射注册表、compat mapper 或仅包装 `contracterrors.FromError` 的权限错误兼容函数
