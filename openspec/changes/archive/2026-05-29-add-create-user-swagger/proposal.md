## Why

当前用户服务仅提供按 ID 查询用户资料接口，缺少创建用户的业务入口，也没有可生成、可访问、与运行时路由保持一致的 Swagger/OpenAPI 文档。引入创建用户接口并统一补齐 API 文档，可以让用户资料能力从只读扩展到可创建，同时为后续接口扩展提供一致的校验、错误返回和文档维护方式。

## What Changes

- 新增 `POST /api/v1/users` 创建用户接口，遵循 controller/service/repository 分层和现有 Fx 组装方式。
- 创建用户请求必须使用 `common/validation` 绑定与校验 JSON 请求体，并补充业务校验，包括必填、格式、长度、枚举/布尔默认、邮箱唯一性与用户已存在冲突处理。
- 复用 `common/response.Envelope`、现有错误码、Zap/trace-id 日志和上下文传递约定，成功创建返回 HTTP 201 与统一响应信封。
- 在当前用户服务中集成 Swagger/OpenAPI 文档生成和访问能力，覆盖新增创建用户接口、现有按 ID 查询用户接口以及健康检查接口。
- 参考 `/Users/liubisen/Desktop/sander/Project/my/go-micro-scaffold/user-services` 的 swaggo 实现方式，但修正其可维护性问题：路由与注解必须一致，生成命令必须匹配当前目录，响应注解必须反映实际 `Envelope` 结构，Swagger 启用策略必须清晰。
- 新增或更新必要的生成命令、依赖、路由注册和生成产物，确保 Swagger 文档可正常生成并通过服务访问。
- 不改变已有 `GET /api/v1/users/:id` 的运行时行为、响应信封或错误码语义。

## Capabilities

### New Capabilities

- `user-profile-create`: 覆盖创建用户 API 的请求校验、业务校验、数据写入、唯一性冲突处理和统一响应契约。
- `api-swagger-documentation`: 覆盖用户服务 Swagger/OpenAPI 元数据、接口注解、文档生成产物、访问路由和生成流程。

### Modified Capabilities

- `user-profile-query`: 为现有 `GET /api/v1/users/:id` 补齐并统一 Swagger 注解与文档输出要求，不改变查询行为。
- `http-service-runtime`: 注册 Swagger UI/文档访问路由，并在用户 API 路由组中新增创建用户路由。
- `api-response-contract`: 明确创建用户成功、参数错误、唯一性冲突、未找到和内部错误在 Swagger 文档中的统一响应信封表达。

## Impact

- API：新增 `POST /api/v1/users`；新增 Swagger UI/文档访问端点；现有 `GET /api/v1/users/:id` 保持兼容。
- 代码：影响 `user-services/internal/controller`、`service`、`repository`、`dto`、`router`、`cmd` 入口注解、bootstrap/Fx wiring、`common/response` 文档模型或示例类型、Swagger 生成目录。
- 数据：复用现有 `users` 表、Ent `User` schema 和唯一邮箱索引；如实现发现 schema 与业务校验不一致，仅在必要时通过 Ent schema 与 Atlas migration 进行最小调整。
- 依赖：引入 swaggo 相关依赖与生成工具使用方式，例如 `github.com/swaggo/swag`、`github.com/swaggo/gin-swagger`、`github.com/swaggo/files`。
- 运维/开发：补充或更新 Swagger 生成命令，确保在 `user-services` 模块内可执行并生成 `docs` 产物；测试仍分别在 `common/` 与 `user-services/` 模块执行。
