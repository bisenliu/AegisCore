## ADDED Requirements

### Requirement: 密码 KDF 显式实例化

系统 MUST 在 `common/security/password` 中提供可显式实例化的 Argon2id 密码哈希与校验 primitive。调用方 MUST 通过实例方法执行密码哈希和校验，并 MUST 在构造实例时声明本实例的 Argon2id 并发上限和队列上限。`common/security/password` MUST NOT 暴露包级密码哈希、包级密码校验或包级可变 Argon2id 门控入口。

#### Scenario: 创建密码 KDF 服务实例

- **WHEN** 服务、CLI 或测试需要执行密码哈希或校验
- **THEN** 调用方 MUST 显式创建 `common/security/password` 的密码 KDF 服务实例
- **AND** 构造参数 MUST 包含正数 Argon2id 并发上限和正数队列上限
- **AND** 队列上限 MUST 大于或等于并发上限

#### Scenario: 拒绝无效 KDF 资源预算

- **WHEN** 调用方使用非正数并发上限、非正数队列上限或小于并发上限的队列上限创建密码 KDF 服务
- **THEN** 系统 MUST 返回明确错误并拒绝创建实例

#### Scenario: 通过实例执行密码哈希

- **WHEN** 调用方使用密码 KDF 服务实例对合法明文密码执行哈希
- **THEN** 系统 MUST 使用 Argon2id 当前安全参数生成包含算法、版本、内存、迭代、并行度、盐和派生密钥的编码哈希
- **AND** 系统 MUST 使用该实例的队列和并发预算限制本实例内执行中和等待中的 KDF 请求

#### Scenario: 通过实例执行密码校验

- **WHEN** 调用方使用密码 KDF 服务实例校验合法明文密码和受支持的编码哈希
- **THEN** 系统 MUST 解析编码哈希中的算法、版本和参数
- **AND** 系统 MUST 只接受当前策略允许的 Argon2id 参数
- **AND** 系统 MUST 使用常量时间比较返回密码是否匹配

#### Scenario: KDF 门控只属于实例

- **WHEN** 多个服务组件、CLI 或测试在同一进程内需要不同密码 KDF 资源预算
- **THEN** 系统 MUST 允许它们持有不同密码 KDF 服务实例
- **AND** 一个实例的队列和并发占用 MUST NOT 消耗另一个实例的队列和并发预算
