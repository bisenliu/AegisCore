## Why

`common` 模块已经承载配置、日志、基础设施、HTTP 中间件、响应契约、认证凭证、密码、时区和请求校验等多个共享能力。虽然现有子目录仍较清晰，但顶层 `common` 命名缺少能力边界约束，后续跨服务代码如果继续无差别沉淀，容易演变为共享杂物模块，增加能力归属判断成本并扩大公共 API 变更影响面。

当前仓库已经通过 capability map 和 OpenSpec 将共享能力拆分为 `shared-infrastructure`、`api-response-contract`、`request-validation`、`common-credentials` 等长期能力，因此现在适合通过目录结构重构让 Go 代码组织与能力边界对齐，并为后续共享能力准入提供更明确的位置语义。

## What Changes

- 将 `common` 下现有共享包按能力边界重组为更明确的顶层分类目录：`contract/`、`runtime/`、`http/`、`security/` 和 `validation/`。
- 将响应契约移动到 `common/contract/response/`，表达其属于跨服务 API contract，而不是普通工具包。
- 将配置、日志、基础设施和时区移动到 `common/runtime/`，表达其属于服务运行时基础能力。
- 将 HTTP 中间件和 Gin 校验适配层移动到 `common/http/`，表达其依赖 HTTP/Gin 边界。
- 将认证凭证原语和密码能力移动到 `common/security/`，表达其属于安全和凭证基础能力。
- 保留通用校验核心在 `common/validation/`，使其继续与 Gin HTTP 适配层分离。
- 同步更新 Go imports、测试、文档、capability map 和相关 OpenSpec 规格引用。
- 保持外部运行时契约不变：HTTP 路径、响应 JSON 字段、业务错误码、配置 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例、Fx named injection、`X-Trace-ID` header 和日志 `trace-id` 字段均不改变。
- 本变更不拆分 Go module，不修改 `github.com/aegiscore/common` module path，不新增业务能力。

## Capabilities

### New Capabilities
- `common-module-organization`: 定义 `common` 模块目录组织、共享能力准入边界、包迁移兼容要求和后续新增共享能力的归属规则。

### Modified Capabilities
- `shared-infrastructure`: 更新配置、日志、基础设施、时区、HTTP middleware 和 Gin adapter 的代码位置要求，保持运行时行为和外部契约不变。
- `api-response-contract`: 更新响应契约包的代码位置要求，保持响应信封、错误码、错误映射和 JSON 字段不变。
- `request-validation`: 更新通用校验核心与 Gin HTTP 适配层的代码位置关系，保持请求绑定、校验错误和失败响应行为不变。
- `common-credentials`: 更新认证与密码凭证原语的代码位置要求，保持 JWT、Bearer、认证上下文和密码 hash 行为不变。

## Impact

- 影响代码：`common/security/auth/`、`common/runtime/config/`、`common/http/ginvalidation/`、`common/runtime/infrastructure/`、`common/runtime/logger/`、`common/http/middleware/`、`common/security/password/`、`common/contract/response/`、`common/runtime/timezone/`、`common/validation/` 及所有引用这些包的 `user-services` 代码和测试。
- 影响文档：`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/opsx/CAPABILITY_MAP.md` 和相关 `openspec/specs/*/spec.md` 中的路径引用。
- 兼容性：Go import path 会因子目录迁移而变化，但 module path 保持 `github.com/aegiscore/common`；本仓库内引用必须一次性同步。HTTP API、配置、数据库 schema、migration 历史、响应码和运行时依赖注入行为保持兼容。
- 风险：包路径迁移影响面较大，需要通过 `go test ./...` 分别验证 `common/` 与 `user-services/` 模块，并重点确认 Fx wiring、Swagger 相关引用和 middleware 行为未被目录调整改变。
