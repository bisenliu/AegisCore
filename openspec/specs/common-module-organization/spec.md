# common-module-organization

## Purpose

common 模块目录组织能力定义跨服务共享代码的能力分类边界，确保共享契约、运行时基础能力、HTTP 适配、安全凭证原语和通用校验核心按长期职责归位。
## Requirements
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

### Requirement: Gate new shared packages by capability ownership
系统 SHALL 要求新增 `common` 共享包先明确 capability ownership 和目录类别。新增共享代码 MUST 属于跨服务稳定契约、运行时基础能力、HTTP 适配、安全凭证原语或通用校验核心之一；服务独有规则 MUST 保持在对应服务模块内，除非已经成为多个服务稳定复用的通用能力。

#### Scenario: Add generic shared capability
- **WHEN** 开发者准备新增可被多个服务复用的共享能力
- **THEN** 开发者 MUST 在 capability map 或 OpenSpec 中明确该能力归属
- **THEN** 代码 MUST 放入与能力类别匹配的 `common` 子目录

#### Scenario: Reject service-specific helper
- **WHEN** 某段逻辑只服务于用户服务的请求清洗、用户状态规则、用户 DTO 映射或用户 repository 行为
- **THEN** 该逻辑 MUST 保持在 `user-services` 的对应边界内
- **THEN** 实现 MUST NOT 仅因未来可能复用而移动到 `common`

### Requirement: Restrict service-local shared directory usage
系统 SHALL 将 `user-services/internal/shared` 作为默认不创建的例外目录管理。只有无法通过 ports 或依赖注入解决，且多个服务内能力必须稳定共享的原子级 Value Object 或极少量跨能力错误定义，才允许进入 `internal/shared`。业务逻辑、工具函数、流程编排、store、service、controller 和 DTO MUST NOT 放入 `internal/shared`。

#### Scenario: Reject shared directory for business helpers
- **Given** 开发者准备新增用户服务内部通用 helper、业务流程、DTO、controller、service 或 store 代码
- **When** 该代码只服务于单一能力，或可以通过能力本地 package、ports 或依赖注入表达协作
- **Then** 实现 MUST NOT 创建或使用 `user-services/internal/shared`
- **Then** 代码 MUST 保持在所属能力目录或明确的运行时边界内

#### Scenario: Allow atomic shared value object after review
- **Given** 多个能力必须共享同一个稳定的原子级 value object，例如服务内跨能力统一的用户 ID 类型
- **When** 该类型无法通过 ports 或依赖注入避免直接共享
- **Then** 开发者 MAY 在 `user-services/internal/shared` 下新增最小内容
- **Then** 变更说明 MUST 解释为什么不能通过 ports 或依赖注入解决、为什么属于多个能力稳定共享、为什么是原子级基础语义而非业务能力下沉

#### Scenario: Preserve common and service-local boundaries
- **Given** 开发者准备新增共享代码
- **When** 该代码属于跨服务稳定契约、运行时基础能力、HTTP 适配、安全凭证原语或通用校验核心
- **Then** 代码 MUST 进入 `common` 对应能力目录
- **Then** 只对用户服务有效的规则 MUST 留在 `user-services` 对应能力边界内

### Requirement: Preserve external contracts during common reorganization
系统 SHALL 在 `common` 目录重组期间保持外部可观察契约不变。实现 MUST 不改变 HTTP 路径、响应 JSON 字段、业务错误码、配置 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例、Fx named injection、`X-Trace-ID` header、日志 `trace-id` 字段、数据库 schema 或 migration 历史。

#### Scenario: Preserve runtime and API contracts
- **WHEN** `common` 包路径被迁移到新的目录结构
- **THEN** HTTP API 响应 MUST 继续使用既有信封字段和错误码
- **THEN** 配置加载 MUST 继续使用既有 YAML key 和 `AEGISCORE_` 环境变量覆盖规则
- **THEN** Redis/PostgreSQL 和 Fx named injection 名称 MUST 保持不变

#### Scenario: Preserve Go module boundary
- **WHEN** `common` 目录结构被重组
- **THEN** `common/go.mod` 的 module path MUST 保持为 `github.com/aegiscore/common`
- **THEN** 本变更 MUST NOT 拆分或重命名 Go module
