## MODIFIED Requirements

### Requirement: 共享安全原语

系统 MUST 在 `common/security` 中提供业务中立的 JWT 验证、Bearer token 处理、Casbin 请求三元组授权和 bcrypt 密码哈希原语，MUST NOT 固定 user-service 的 claims schema、token subject、会话撤销或业务授权模型。

#### Scenario: JWT middleware 使用最小 verifier

- **WHEN** 服务创建共享 JWT 认证 middleware
- **THEN** middleware constructor MUST 只接收 logger、访问令牌 verifier 和可选 token version validator
- **AND** middleware MUST NOT 依赖 token issuer、服务私有配置或具备签发能力的 concrete service
- **AND** access token claims、subject 和业务字段校验 MUST 由服务私有 verifier adapter 拥有
- **AND** `common/security/auth` MUST NOT 提供 access、refresh 或 password-change token 签发入口，也 MUST NOT 定义 user-service 专属 subject 或 claims

#### Scenario: Casbin 授权入口

- **WHEN** 调用方需要获得 Casbin 三元组授权的原始结果
- **THEN** `common/security/casbin.Enforce` MUST 返回 `bool` 和 `error`
- **AND** 拒绝访问到 `ErrDenied` 的转换 MUST 由 `Authorizer.Authorize` 或调用方显式处理

#### Scenario: bcrypt 密码哈希和校验

- **WHEN** 服务、CLI 或测试需要执行密码哈希或校验
- **THEN** 调用方 MUST 显式创建 bcrypt 密码服务实例
- **AND** `common/security/password` MUST 使用固定 bcrypt cost 生成新密码哈希，初始 cost MUST 为 `12`
- **AND** `common/security/password` MUST 使用 bcrypt 校验已编码的密码哈希，非 bcrypt、格式非法或无法解析的哈希 MUST 被拒绝
- **AND** `common/security/password` MUST NOT 验证、迁移、fallback 或 rehash Argon2id 密码哈希
- **AND** `common/security/password` MUST NOT 暴露包级哈希、校验或可变算法配置入口

#### Scenario: 明文密码长度边界

- **WHEN** 调用方提交空明文密码或超过 bcrypt 安全输入上限的明文密码
- **THEN** `common/security/password` MUST 在执行 bcrypt 前拒绝该输入
- **AND** 空密码 MUST 返回可匹配的 `password.ErrEmptyPassword`
- **AND** 超长密码 MUST 返回可匹配的 `password.ErrPasswordTooLong`

### Requirement: Runtime 配置加载与服务配置边界

系统 MUST 在 `common/runtime/config` 中维护跨服务 runtime 配置、默认值和通用校验。服务私有业务配置、必需资源名、业务用途和配置 map 到真实资源的选择 MUST 由消费服务拥有。

#### Scenario: 严格加载通用配置

- **WHEN** 服务通过配置文件启动
- **THEN** 共享 loader MUST 解析 runtime、HTTP、gRPC、metrics、tracing、pprof、logger 和通用 `local_cache` 配置
- **AND** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 的 decode 能力解析 duration、slice 和具名配置
- **AND** 未声明字段 MUST 在启动前失败并报告完整路径，不得使用旧字段别名或 fallback

#### Scenario: 通用 runtime 字段和安全校验

- **WHEN** 服务加载 runtime 配置
- **THEN** 共享 runtime config MUST 声明并校验 `runtime.gin.mode`、server、logger、metrics、tracing、pprof、lifecycle 和通用 local cache 配置
- **AND** `runtime.gin.mode` 默认值 MUST 为 `release`，环境变量覆盖 MUST 使用 `AEGISCORE_RUNTIME_GIN_MODE`，合法值 MUST 仅为 `debug`、`release` 或 `test`
- **AND** `observability.pprof.enabled` 和 `observability.pprof.addr` 默认值 MUST 分别为 `false` 和 `127.0.0.1:6060`
- **AND** production-like 环境启用 pprof 时 `observability.pprof.addr` MUST 使用 loopback host
- **AND** 至少一个 HTTP 或 gRPC server MUST 启用

#### Scenario: 服务私有配置留在服务边界

- **WHEN** 服务需要 `auth`、`ent`、JWT TTL、refresh session、token version、RBAC 或 production-like secret 校验
- **THEN** 服务私有 loader MUST 负责解析和校验这些配置
- **AND** `common/runtime/config` MUST NOT 声明或校验这些业务配置
- **AND** 服务私有配置 MUST NOT 声明、读取或兼容旧 `auth.password_kdf` 配置

#### Scenario: 通用具名本地缓存配置

- **WHEN** 配置包含 `local_cache.<name>`
- **THEN** loader MUST 保留 `<name>` 并解析为通用缓存实例配置
- **AND** validation MUST 校验 `capacity > 0`、`ttl > 0`、`load_timeout > 0`、`num_counters >= 0` 和 `buffer_items >= 0`，错误 MUST 包含完整字段路径
- **AND** 必需缓存名及其业务含义 MUST 留在消费服务

#### Scenario: Runtime lifecycle 停止预算校验

- **WHEN** 配置中的 `runtime.lifecycle.stop_timeout` 小于 HTTP shutdown timeout、worker drain allowance、tracing flush allowance 和 shutdown safety margin 的组合最低预算
- **THEN** 配置校验 MUST 失败并指出 `runtime.lifecycle.stop_timeout` 以及最低所需预算
- **WHEN** 配置中的 `runtime.lifecycle.stop_timeout` 大于或等于组合最低预算
- **THEN** 共享 runtime 配置校验 MUST 继续通过，且业务停止策略 MUST 由 owning feature 或服务组合层表达
