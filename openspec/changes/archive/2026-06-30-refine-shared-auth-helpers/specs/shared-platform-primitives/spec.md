## ADDED Requirements

### Requirement: 共享认证授权 helper API 治理

系统 MUST 在 `common/http` 和 `common/security` 中保持认证、授权 helper 的导出 API 语义清晰且避免重复简写入口；当共享 helper 只包装另一个推荐入口且没有额外稳定语义时，系统 MUST 通过显式推荐入口、废弃标记或移除策略治理该 helper。

#### Scenario: Casbin 授权 helper 收紧

- **WHEN** 调用方需要获得 Casbin 三元组授权的原始允许结果
- **THEN** 系统 MUST 提供 `common/security/casbin.Enforce` 作为返回 `bool` 和 `error` 的推荐入口
- **AND** 拒绝访问转换为 `ErrDenied` 的 error-only 语义 MUST 由 `Authorizer.Authorize` 或调用方显式处理

#### Scenario: JWT middleware 无 token version 校验

- **WHEN** 服务需要创建不执行 token version 撤销校验的 JWT 认证中间件
- **THEN** 系统 MUST 推荐调用 `AuthWithTokenVersionValidator(log, jwtService, cfg, nil)` 显式表达该行为
- **AND** 仅作为兼容保留的简写 helper MUST 标记为废弃或在确认无消费者后移除

#### Scenario: 行为保持不变

- **WHEN** 共享认证授权 helper 的重复入口被废弃或移除
- **THEN** 系统 MUST 保持 JWT 解析、token version 校验、Casbin 三元组校验、`ErrNotConfigured`、`ErrDenied` 和 HTTP 响应语义不变
- **AND** user-service 的认证路由挂载和 RBAC 保护路由 MUST 不因该 API 治理发生行为变化
