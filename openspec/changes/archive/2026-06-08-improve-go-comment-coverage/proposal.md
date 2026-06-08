## Why

当前 Go 代码存在公开 API 注释缺失、注释规范不一致以及复杂逻辑缺少解释的问题，降低了跨模块维护、代码审查和 lint 自动化治理的效率。现在需要通过一次集中整改建立可验证的注释基线，避免后续能力扩展时继续累积不可维护的隐性知识。

## What Changes

- 为 Go 代码中所有导出的类型、函数、方法、接口、常量和变量补充符合 Go godoc 规范的注释。
- 为复杂业务逻辑、关键分支、边界条件、不直观实现和策略参数补充必要注释，重点解释设计原因和业务含义。
- 通过 `golangci-lint` 检查并修复与注释相关的问题，确保注释覆盖和规范性可被自动化验证。
- 不修改 HTTP API、错误码、配置格式、数据库 schema、迁移流程或运行时行为。

## Capabilities

### New Capabilities
- `go-comment-governance`: 约束 Go 代码公开 API、复杂逻辑和策略参数的注释规范，并要求通过 lint 维持可维护的注释基线。

### Modified Capabilities
- 无。

## Impact

- 影响范围覆盖 `common/` 和 `user-services/` 中的 Go 源码，包括当前能力地图中的 `user-profile-query`、`user-profile-create`、`user-authentication`、`user-session-control`、`http-service-runtime`、`shared-infrastructure`、`api-response-contract`、`request-validation`、`common-module-organization`、`api-swagger-documentation`、`database-schema-migrations` 和 `go-toolchain-baseline` 相关实现文件。
- 不产生外部可观察行为变更；客户端 API、响应信封、错误码、配置键、数据库结构和生成代码语义保持兼容。
- 需要在实现阶段运行 `golangci-lint`，并根据 lint 输出修复注释相关问题。
