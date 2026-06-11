## ADDED Requirements

### Requirement: Distinguish token version mismatch from infrastructure failures

用户会话控制能力 SHALL 在 Access Token 的 token version 校验中区分安全拒绝与基础设施故障。认证中间件使用的 token version validator MUST 仅在服务端当前 `token_version` 与 token claims 中的 `token_version` 不一致时返回 token version mismatch 语义；Redis token version cache 读取失败、PostgreSQL 回源失败、缓存回填失败或其他非预期依赖错误 MUST 保留为基础设施错误，不得伪装为 `ErrTokenVersionMismatch`、`ErrTokenInvalid` 或等价 token invalid 语义。基础设施错误发生时系统 MUST fail-closed，业务 handler MUST NOT 执行。

#### Scenario: Mismatched token version rejects as authentication failure
- **Given** Access Token 签名有效且未过期
- **Given** token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 认证中间件执行 token version 校验
- **Then** token version validator MUST 返回 token version mismatch 语义
- **Then** 系统 MUST 拒绝请求并返回 token invalid 或等价认证失败响应
- **Then** 受保护业务 handler MUST NOT 执行

#### Scenario: Redis token version cache failure remains infrastructure error
- **Given** Access Token 签名有效且未过期
- **Given** Redis token version cache 读取发生非 cache miss 的连接、超时或协议错误
- **When** 认证中间件执行 token version 校验
- **Then** token version validator MUST NOT 返回 token version mismatch 或 token invalid 语义
- **Then** 系统 MUST 将该错误保留为基础设施错误
- **Then** 系统 MUST fail-closed 并阻止受保护业务 handler 执行

#### Scenario: Database fallback failure remains infrastructure error
- **Given** Access Token 签名有效且未过期
- **Given** Redis token version cache miss 后 PostgreSQL 回源读取当前 `token_version` 失败
- **When** 认证中间件执行 token version 校验
- **Then** token version validator MUST NOT 将该 PostgreSQL 错误映射为 token invalid
- **Then** 系统 MUST 将该错误保留为基础设施错误
- **Then** 系统 MUST fail-closed 并阻止受保护业务 handler 执行

#### Scenario: Cache backfill failure remains infrastructure error
- **Given** Access Token 签名有效且未过期
- **Given** Redis token version cache miss 后 PostgreSQL 已成功读取当前 `token_version`
- **Given** Redis token version cache 回填失败
- **When** 认证中间件执行 token version 校验
- **Then** token version validator MUST NOT 将回填失败静默吞掉或映射为 token invalid
- **Then** 系统 MUST 将该错误保留为基础设施错误
- **Then** 系统 MUST fail-closed 并阻止受保护业务 handler 执行

#### Scenario: Infrastructure error is observable without sensitive token data
- **Given** token version 校验发生 Redis 或 PostgreSQL 基础设施错误
- **When** 系统记录该错误
- **Then** 日志 MUST 使用 error 级别或等价告警级别
- **Then** 日志 MUST 保留底层错误上下文、`trace-id` 和非敏感 `user_id`
- **Then** 日志 MUST NOT 包含原始 JWT、Refresh Token、密码、完整 claims 或签名材料
