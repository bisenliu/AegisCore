## ADDED Requirements

### Requirement: Feature-local role 应用错误契约

系统 MUST 允许 user-service feature-local role domain 为稳定角色和 RBAC 绑定错误定义应用错误，使这些错误同时携带 `Kind`、`Reason`、`Code`、`Message` 并可被共享 response helper 直接渲染。该契约 MUST 保持业务归属清晰，且不得把 role、permission 或 identity 的错误映射表上移到 `common`、`internal/shared` 或跨 feature 全局包。

#### Scenario: role domain 定义可渲染应用错误

- **WHEN** `user-service/internal/features/role/domain` 定义角色目录、用户角色绑定或角色权限绑定错误
- **THEN** 错误 MUST 携带共享错误契约所需的 `Kind`、稳定 `Reason`、`Code` 和中文公开 `Message`
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误

#### Scenario: role domain 保持业务判断语义

- **WHEN** 调用方通过 `errors.Is` 判断 role domain 导出的稳定业务错误
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

#### Scenario: 消费方 feature 透传应用错误

- **WHEN** role feature 接收到 `identity` 或 `permission` 边界拥有的应用错误
- **THEN** role feature MAY 返回或包装该错误并交给共享 response helper 渲染
- **AND** role HTTP transport MUST NOT 为这些跨 feature 错误维护重复 sentinel-to-HTTP 映射

#### Scenario: common 保持业务中立

- **WHEN** 角色与 RBAC 绑定错误迁移为应用错误
- **THEN** `common/` MUST 只提供业务中立的错误契约、应用错误构造和 response 渲染 helper
- **AND** 系统 MUST NOT 在 `common` 新增角色、用户角色绑定、角色权限绑定、permission 或 identity 专用 `Reason` 常量、公开消息、错误变量或错误到 HTTP 响应的全局映射表

#### Scenario: 不新增跨模块角色错误注册表

- **WHEN** feature-local role 应用错误或跨 feature 应用错误需要被 role HTTP controller 渲染
- **THEN** controller MUST 通过共享 `response.Fail` 和错误自身携带的契约信息完成渲染
- **AND** 系统 MUST NOT 新增跨模块角色错误映射注册表、compat mapper 或仅包装 `contracterrors.FromError` 的角色错误兼容函数
