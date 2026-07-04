## MODIFIED Requirements

### Requirement: 共享认证授权 helper API 治理

系统 MUST 在 `common/http` 和 `common/security` 中保持认证、授权 helper 的导出 API 语义清晰且避免重复简写入口；当共享 helper 只包装另一个推荐入口、暴露未参与行为的参数或没有额外稳定语义时，系统 MUST 通过显式推荐入口、废弃标记或移除策略治理该 helper。

#### Scenario: Casbin 授权 helper 收紧

- **WHEN** 调用方需要获得 Casbin 三元组授权的原始允许结果
- **THEN** 系统 MUST 提供 `common/security/casbin.Enforce` 作为返回 `bool` 和 `error` 的推荐入口
- **AND** 拒绝访问转换为 `ErrDenied` 的 error-only 语义 MUST 由 `Authorizer.Authorize` 或调用方显式处理

#### Scenario: JWT middleware 无 token version 校验

- **WHEN** 服务需要创建不执行 token version 撤销校验的 JWT 认证中间件
- **THEN** 系统 MUST 推荐调用 `AuthWithTokenVersionValidator(log, jwtService, nil)` 显式表达该行为
- **AND** 仅作为兼容保留的简写 helper MUST 标记为废弃或在确认无消费者后移除

#### Scenario: JWT middleware 不接收无效配置参数

- **WHEN** 服务需要创建共享 JWT 认证 middleware
- **THEN** `AuthWithTokenVersionValidator` MUST 只接收 logger、JWT service 和可选 token version validator 作为调用参数
- **AND** `AuthWithTokenVersionValidator` MUST NOT 接收 `config.AuthConfig` 或其他不参与运行时认证行为的配置参数
- **AND** JWT 配置 MUST 由 `auth.NewJWTService(config.AuthConfig)` 消费后通过 `JWTService` 注入 middleware

#### Scenario: token version validator 函数适配器移除

- **WHEN** 服务需要为共享 JWT 认证 middleware 提供 token version 撤销校验
- **THEN** 调用方 MUST 直接提供实现 `common/security/auth.TokenVersionValidator` 的具体类型
- **AND** `common/http/middleware` MUST NOT 暴露只将函数包装为 `TokenVersionValidator` 的 `TokenVersionValidatorFunc` 适配器

#### Scenario: 行为保持不变

- **WHEN** 共享认证授权 helper 的重复入口或无效参数被废弃或移除
- **THEN** 系统 MUST 保持 JWT 解析、token version 校验、Casbin 三元组校验、`ErrNotConfigured`、`ErrDenied` 和 HTTP 响应语义不变
- **AND** user-service 的认证路由挂载和 RBAC 保护路由 MUST 不因该 API 治理发生行为变化
