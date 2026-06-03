## Why

当前用户服务的部分基础代码已经承担多类职责：`common/response/response.go` 同时包含响应信封、分页和错误响应 helper，`user-services/internal/bootstrap/bootstrap.go` 同时组装 Fx、Gin、JWT、路由、HTTP server 和资源依赖，`user-services/ent/schema/` 也仍是单层 schema 文件布局。随着 `organization`、`role`、`permission` 等实体和更多运行时能力加入，这些职责混杂会增加维护成本和变更风险。

## What Changes

- 为 Ent schema 目录引入按领域分类的实际代码组织，将当前用户 schema 纳入用户领域分类，并同步调整 Ent 生成与 Atlas schema source 以保持可用。
- 拆分 `common/response` 内的响应信封、分页模型/计算和错误响应 helper，保持现有导出 API 与 JSON 响应契约不变。
- 拆分 `user-services/internal/bootstrap` 内的运行时组装职责，将 Fx app/module、Gin engine、JWT provider、路由注册、HTTP server lifecycle 和资源装配放入更清晰的文件边界。
- 保持 HTTP API、错误码、响应 JSON 结构、配置键、数据库表结构和 migration 语义不变；本变更不新增业务功能。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `database-schema-migrations`: 将 Ent schema 源文件实际调整为领域分类组织，支持未来多领域实体扩展但不改变当前数据库结构。
- `api-response-contract`: 保持统一响应契约不变，同时要求响应、分页和错误 helper 在代码组织上职责清晰。
- `http-service-runtime`: 保持服务启动、Gin 中间件、路由注册和优雅关闭行为不变，同时拆分 bootstrap 运行时职责边界。
- `shared-infrastructure`: 保持配置、日志、Redis/PostgreSQL/Ent 命名实例装配不变，同时让用户服务资源装配在 bootstrap 中独立表达。

## Impact

- 影响代码位置：`user-services/ent/schema/`、`common/response/`、`user-services/internal/bootstrap/`。
- API 兼容性：不改变现有 HTTP 路由、请求参数、响应信封字段、错误码或错误消息语义。
- 数据兼容性：调整 Ent schema 源文件组织但不改变 Ent `User` 字段、索引、Atlas migration 或数据库表结构。
- 配置兼容性：不改变 YAML 配置结构、`AEGISCORE_` 环境变量覆盖规则或命名实例键。
- 依赖影响：不引入新的第三方依赖。
