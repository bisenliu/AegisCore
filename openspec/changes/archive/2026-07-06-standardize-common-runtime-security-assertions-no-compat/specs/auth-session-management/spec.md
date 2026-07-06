## ADDED Requirements

### Requirement: shared auth 和 password 测试断言迁移

`common/security/auth` 和 `common/security/password` 的测试 MUST 使用统一断言规范验证 JWT、token subject、token version、password KDF、密码哈希和密码校验行为。断言迁移 MUST 保持 JWT claims 解析、token subject 校验、token version 校验、Argon2id 参数、KDF 资源预算、队列繁忙错误、密码哈希编码和常量时间校验语义不变。

#### Scenario: JWT 和 token 断言

- **WHEN** `common/security/auth` 测试验证 token 签发、解析、过期、subject、`jti`、token version 或错误路径
- **THEN** 测试 MUST 使用 `require` 表达错误、claims、时间、subject 和版本匹配断言
- **AND** 迁移 MUST NOT 改变 token 格式、claims 名称、过期校验或 subject 隔离语义

#### Scenario: password KDF 断言

- **WHEN** `common/security/password` 测试验证 Argon2id 哈希、校验、参数解析、资源预算或队列繁忙路径
- **THEN** 测试 MUST 使用 `require` 表达构造错误、哈希格式、校验结果、错误类型和资源边界断言
- **AND** 迁移 MUST NOT 改变 Argon2id 参数、哈希编码、队列上限、并发上限或 `ErrPasswordKDFBusy` 语义

#### Scenario: 安全失败路径不放宽

- **WHEN** auth 或 password 测试迁移历史 `t.Fatal`、`t.Error` 手写判断
- **THEN** 测试 MUST 保持原有安全失败路径覆盖
- **AND** 迁移 MUST NOT 通过兼容 helper、生产分支或弱化断言使非法 token、错误密码、过期 token 或资源繁忙路径被放行
