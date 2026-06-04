## MODIFIED Requirements

### Requirement: Organize common packages by shared capability category
系统 SHALL 将 `common` 模块内共享包按长期能力类别组织到明确目录边界中。响应契约 MUST 位于 `common/contract/response`；配置、配置 Fx adapter、日志、日志 Fx adapter、datastore 纯构造、datastore Fx adapter、运行时资源名和时区 runtime 能力 MUST 位于 `common/runtime` 下的职责明确子包；HTTP 中间件和 Gin 校验适配层 MUST 位于 `common/http` 下；认证与密码凭证原语 MUST 位于 `common/security` 下；通用校验核心 MUST 保持在 `common/validation`。

#### Scenario: Locate response contract
- **WHEN** 维护者需要修改统一响应信封、业务错误码、失败响应 helper 或分页响应模型
- **THEN** 相关代码 MUST 位于 `common/contract/response`
- **THEN** 维护者 MUST NOT 将响应契约实现放入无能力语义的顶层 `common/response`

#### Scenario: Locate runtime capabilities
- **WHEN** 维护者需要修改配置加载、配置 Fx provider、Zap logger、logger Fx provider、Redis/PostgreSQL 构造、Redis/PostgreSQL Fx provider、运行时资源名或 timezone 初始化
- **THEN** 相关代码 MUST 分别位于 `common/runtime/config`、`common/runtime/configfx`、`common/runtime/logger`、`common/runtime/loggerfx`、`common/runtime/datastore`、`common/runtime/datastorefx`、`common/runtime/resources` 或 `common/runtime/timezone`
- **THEN** 这些 runtime 包 MUST NOT 承载用户服务业务 controller、service、repository 或 DTO 逻辑

#### Scenario: Keep pure runtime packages independent from Fx adapters
- **WHEN** 维护者修改 `common/runtime/config`、`common/runtime/logger` 或 `common/runtime/datastore`
- **THEN** 这些纯逻辑包 MUST NOT import `go.uber.org/fx`
- **THEN** Fx provider、Fx lifecycle hook 和 Fx result tag 相关代码 MUST 位于对应 `configfx`、`loggerfx` 或 `datastorefx` adapter 包

#### Scenario: Locate HTTP adapters
- **WHEN** 维护者需要修改 Gin middleware、trace-id middleware、recovery middleware、CORS middleware、request logging middleware 或 Gin validation adapter
- **THEN** 相关代码 MUST 位于 `common/http/middleware` 或 `common/http/ginvalidation`
- **THEN** HTTP adapter 包 MUST NOT 吸收非 HTTP 的通用 validation core 实现

#### Scenario: Locate security primitives
- **WHEN** 维护者需要修改 JWT token primitive、Bearer 传输常量、认证上下文 helper、Argon2id 密码 hash 或密码校验
- **THEN** 相关代码 MUST 位于 `common/security/auth` 或 `common/security/password`
- **THEN** security 包 MUST NOT 承载用户服务登录、刷新、登出、session repository 或业务认证编排逻辑
