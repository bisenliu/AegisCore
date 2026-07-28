## ADDED Requirements

### Requirement: 登录审计客户端地址

系统 MUST 在登录流程中将 Gin trusted proxy 校验后的 `c.ClientIP()` 写入认证审计上下文。登录 controller MUST 保持 `binding.BindOrAbort`、input preparer、use case、`response.OK` 或 `response.Fail` 的职责边界，MUST NOT 在 controller、input preparer 或 auth application 中手写解析 forwarded headers。

#### Scenario: 受信任代理后的登录审计

- **WHEN** 登录请求来自 `server.http.trusted_proxies` 显式信任的上游代理，且代理提供已清洗的 forwarded headers
- **THEN** auth controller MUST 将 `c.ClientIP()` 解析出的真实客户端地址写入 `authctx.ClientContext.ClientIP`
- **AND** 登录失败、密码不匹配或用户状态拒绝的认证安全日志 MUST 使用该客户端地址

#### Scenario: 未受信任来源的登录审计

- **WHEN** 登录请求来自未配置为 trusted proxy 的 TCP peer，即使请求携带 `X-Forwarded-For` 或 `X-Real-IP`
- **THEN** auth controller MUST 将 TCP peer 地址写入 `authctx.ClientContext.ClientIP`
- **AND** 系统 MUST NOT 信任或记录未受信任 peer 提供的 forwarded client IP
