## ADDED Requirements

### Requirement: Password KDF busy 应用错误契约

`common/security/password` MUST 将密码 KDF 资源繁忙表达为业务中立的应用错误，使调用方可通过共享 `response.Fail` 直接渲染为服务不可用响应，同时保持 Argon2id 参数、哈希编码、并发上限、队列上限和 `errors.Is(err, password.ErrPasswordKDFBusy)` 语义不变。

#### Scenario: KDF busy 直接渲染为服务不可用

- **WHEN** `common/security/password` 的 KDF 服务实例因为执行中和等待中的请求数达到实例资源预算而返回 `password.ErrPasswordKDFBusy`
- **THEN** 该错误 MUST 携带 `KindServiceUnavailable`、稳定 `Reason` 值 `password_kdf_busy`、`CodeServiceUnavailable` 和不泄露资源预算的中文公开 message
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误为 `503 Service Unavailable`
- **AND** 该错误 MUST NOT 要求 user-service auth HTTP mapper 才能获得服务不可用语义

#### Scenario: KDF busy 保持 errors.Is 语义

- **WHEN** 调用方通过 `errors.Is(err, password.ErrPasswordKDFBusy)` 判断 KDF 资源繁忙
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

#### Scenario: password primitive 保持业务中立

- **WHEN** `common/security/password` 定义 `password.ErrPasswordKDFBusy`
- **THEN** `common` MUST 只表达密码 KDF 资源预算繁忙这一业务中立语义
- **AND** `common` MUST NOT 承载 user-service 登录、用户名、认证会话、token、强制改密、撤销或认证公开消息以外的业务编排逻辑

#### Scenario: KDF 安全语义不变

- **WHEN** password KDF busy 错误迁移为应用错误
- **THEN** 系统 MUST NOT 改变 Argon2id 参数、哈希编码、队列上限、并发上限、常量时间校验或资源预算触发条件
- **AND** 测试 MUST 继续覆盖队列繁忙路径、哈希成功路径和密码校验失败路径

### Requirement: Feature-local auth domain 应用错误契约

系统 MUST 允许 user-service auth domain 为稳定认证业务错误定义应用错误，使这些错误同时携带 `Kind`、`Reason`、`Code`、`Message` 并可被共享 response helper 直接渲染。该契约 MUST 保持业务归属清晰，且不得把 user-service 的认证错误映射表上移到 `common`、`internal/shared` 或跨 feature 全局包。

#### Scenario: auth domain 定义可渲染应用错误

- **WHEN** `user-service/internal/features/auth/domain` 定义无效凭据、缺失会话、token 无效、refresh session 无效、强制改密 session 无效或撤销不完整错误
- **THEN** 错误 MUST 携带共享错误契约所需的 `Kind`、稳定 `Reason`、`Code` 和中文公开 `Message`
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误

#### Scenario: auth domain 保持业务判断语义

- **WHEN** 调用方通过 `errors.Is` 判断 auth domain 导出的稳定业务错误
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

#### Scenario: common 保持认证业务中立

- **WHEN** auth domain 错误迁移为应用错误
- **THEN** `common/` MUST 只提供业务中立的错误契约、应用错误构造、password primitive 和 response 渲染 helper
- **AND** 系统 MUST NOT 在 `common` 新增 user-service 认证专用错误映射表、登录编排、session 语义、token version 语义或 Redis key schema

#### Scenario: 不新增跨模块认证错误注册表

- **WHEN** feature-local auth domain 应用错误需要被 HTTP controller 渲染
- **THEN** controller MUST 通过共享 `response.Fail` 和错误自身携带的契约信息完成渲染
- **AND** 系统 MUST NOT 新增跨模块认证错误映射注册表、compat mapper 或仅包装 `contracterrors.FromError` 的认证错误兼容函数
