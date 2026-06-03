## ADDED Requirements

### Requirement: Provide shared Argon2id password helpers
系统 SHALL 在 `common` 模块提供统一的密码哈希和密码校验方法。密码哈希 MUST 使用 Argon2id、随机盐和可解析的编码格式保存算法版本与参数；密码校验 MUST 使用编码 hash 中的参数重新计算并执行常量时间比较。该能力 MUST 不创建 Redis client、PostgreSQL 连接池、Ent client、HTTP server 或 Fx runtime dependency。

#### Scenario: Hash password with Argon2id
- **Given** 调用方传入非空明文密码
- **When** 调用 `common` 密码哈希方法
- **Then** 系统 MUST 使用 Argon2id 生成密码 hash
- **Then** hash MUST 包含算法标识、版本、内存、迭代次数、并行度、salt 和派生值
- **Then** hash MUST NOT 等于明文密码

#### Scenario: Verify matching password
- **Given** 数据库中保存由 `common` 密码哈希方法生成的 Argon2id hash
- **When** 调用方使用相同明文密码调用 `common` 密码校验方法
- **Then** 系统 MUST 返回校验通过

#### Scenario: Reject non-matching password
- **Given** 数据库中保存由 `common` 密码哈希方法生成的 Argon2id hash
- **When** 调用方使用不同明文密码调用 `common` 密码校验方法
- **Then** 系统 MUST 返回校验不通过
- **Then** 系统 MUST NOT 在错误中公开明文密码、完整 hash、salt 或 hash 参数

#### Scenario: Reject malformed password hash
- **Given** 数据库中的密码 hash 不是 `common` 支持的 Argon2id 编码格式
- **When** 调用方调用 `common` 密码校验方法
- **Then** 系统 MUST 返回校验错误或校验不通过
- **Then** 系统 MUST NOT panic

#### Scenario: Password helper has no runtime datastore side effects
- **Given** 服务或测试引入 `common` 密码 helper
- **When** 调用密码哈希或校验方法
- **Then** 系统 MUST NOT 创建 Redis client、PostgreSQL 连接池、Ent client、HTTP server 或 Fx app
