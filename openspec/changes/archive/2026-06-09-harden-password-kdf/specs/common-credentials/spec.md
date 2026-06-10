## MODIFIED Requirements

### Requirement: Provide password credential primitives
系统 SHALL 在 `common/security/password` 包提供可复用的密码凭证原语，用于生成和校验 Argon2id 密码 hash。密码 hash 输出格式、默认参数、空密码错误和无效 hash 错误语义 MUST 与现有密码凭证行为保持兼容。实现 MUST 使用具名常量表达 encoded hash 最大长度边界，避免在解析逻辑中使用裸字面量。系统 MUST 提供 `HashContext` 和 `VerifyContext` 支持等待 Argon2id KDF 槽位时被 `context.Context` 取消，并 MUST NOT 继续提供旧的 `Hash` 和 `Verify` 同步入口。系统 MUST 限制单进程内 Argon2id KDF 并发数量和执行中/等待中请求总数；当请求总数达到上限时 MUST 返回 KDF busy 错误。系统 MUST 限制明文密码最大长度，并 MUST 拒绝不符合当前策略的 Argon2id 参数、salt 长度或 key 长度。

#### Scenario: Hash password with Argon2id
- **Given** 调用方提供非空且未超过最大长度的明文密码
- **When** 调用方使用 `password.HashContext` 生成密码 hash
- **Then** 系统 MUST 返回 Argon2id 格式的密码 hash
- **Then** hash MUST 包含算法、版本、memory、iterations、parallelism、salt 和 key 信息

#### Scenario: Verify matching password
- **Given** 系统已经通过 `password.HashContext` 生成密码 hash
- **When** 调用方使用相同明文密码调用 `password.VerifyContext`
- **Then** 系统 MUST 返回匹配成功
- **Then** 校验过程 MUST 使用 constant-time comparison 比较派生 key

#### Scenario: Reject invalid password hash
- **Given** 调用方提供格式非法、版本不匹配、参数非法、参数不符合当前策略、salt 长度不符合当前策略、key 长度不符合当前策略或 base64 内容非法的密码 hash
- **When** 调用方使用 `password.VerifyContext` 校验密码
- **Then** 系统 MUST 返回密码 hash 无效错误
- **Then** 系统 MUST NOT 将该密码视为匹配成功

#### Scenario: Password hash length boundary is named
- **Given** 维护者查看 `common/security/password` 的 encoded hash 解析逻辑
- **When** 实现限制 encoded hash 最大长度
- **Then** 系统 MUST 使用具名常量表达最大长度边界
- **Then** 最大长度值 MUST 保持为 `512`

#### Scenario: Reject empty or oversized plain password
- **Given** 调用方提供空明文密码或超过最大长度的明文密码
- **When** 调用方使用 `password.HashContext` 或 `password.VerifyContext`
- **Then** 空明文密码 MUST 返回空密码错误
- **Then** 超过最大长度的明文密码 MUST 返回密码过长错误
- **Then** 系统 MUST NOT 执行 Argon2id KDF

#### Scenario: Remove legacy synchronous password APIs
- **Given** 维护者查看 `common/security/password` 包的公开 API
- **When** 系统完成密码 KDF 强化变更
- **Then** 包 MUST NOT 暴露 `Hash` 同步入口
- **Then** 包 MUST NOT 暴露 `Verify` 同步入口
- **Then** 仓库内调用方 MUST 使用 `HashContext` 或 `VerifyContext`

#### Scenario: Cancel while waiting for password KDF capacity
- **Given** Argon2id KDF 执行槽位不可立即获得
- **Given** 调用方传入的 `context.Context` 在等待期间被取消或超时
- **When** 调用方使用 `password.HashContext` 或 `password.VerifyContext`
- **Then** 系统 MUST 返回 context 取消或超时错误
- **Then** 系统 MUST 释放已占用的等待队列资源

#### Scenario: Reject password KDF when queue is full
- **Given** 单进程内 Argon2id KDF 执行中和等待中的请求总数已达到包内上限
- **When** 新调用方使用 `password.HashContext` 或 `password.VerifyContext` 请求执行 KDF
- **Then** 系统 MUST 返回 KDF busy 错误
- **Then** 系统 MUST NOT 让该请求无限等待或进入 Argon2id KDF
