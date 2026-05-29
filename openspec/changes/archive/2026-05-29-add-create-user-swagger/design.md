## Context

当前仓库是 Go workspace，`common` 提供共享响应、校验、中间件、日志与基础设施，`user-services` 通过 Gin、Fx、Ent 和 Atlas 提供用户服务运行时。现有用户能力只包含 `GET /api/v1/users/:id`，实现路径为 `controller -> service -> repository -> ent.User`，响应统一使用 `common/response.Envelope`。请求校验已经在 `common/validation` 中提供 URI、query、JSON、form binder 和中文错误归一化，当前查询接口已通过该能力绑定 URI 参数。

参考项目 `/Users/liubisen/Desktop/sander/Project/my/go-micro-scaffold/user-services` 使用 swaggo，将全局文档注解放在 server 入口，将 endpoint 注解放在 handler 方法上，并通过专门的 route 文件注册 `/swagger/*any`、`/docs`、`/api-docs`。该方式适合迁移，但参考项目存在路由注解与实际路由不一致、生成命令目录陈旧、健康检查响应注解与真实响应不一致、文档开关说明与代码不完全一致等问题。本变更迁移成熟结构，不迁移这些问题。

## Goals / Non-Goals

**Goals:**

- 新增 `POST /api/v1/users`，并保持 controller/service/repository 分层边界清晰。
- 创建用户请求通过 `common/validation.JSONBinder` 完成请求体绑定和结构体校验。
- 在 service/repository 层完成业务校验和唯一性冲突处理，复用 `common/response` 与 `user-services/internal/apperror`。
- 复用现有 `ent.User` schema、`users` 表和唯一邮箱索引，除非实现时发现现有 schema 不能表达创建用户所需约束。
- 引入 Swagger/OpenAPI 文档能力，覆盖创建用户、查询用户和健康检查，并能在服务启动后访问 Swagger UI。
- 建立可重复的 Swagger 生成流程，生成产物与注解保持一致。

**Non-Goals:**

- 不引入认证、授权、登录态或 Bearer token 保护。
- 不新增用户列表、更新、禁用、删除接口。
- 不改变 `GET /api/v1/users/:id` 的响应字段、HTTP status、错误码或错误文案语义。
- 不在 `common` 中重新实现 Swagger 框架，也不为单个服务复制响应或校验基础能力。
- 不手写 `user-services/ent/` 下生成代码。

## Decisions

### 创建用户沿用现有分层

`UserController` 新增 `Create` 方法，只负责 JSON 绑定、调用 service 和输出 `response.Created` 或 `response.Fail`。`UserService` 新增 `CreateUser`，负责字段规范化、业务校验和错误映射。`UserRepository` 新增邮箱存在检查与创建方法，负责 Ent 查询和写入。

备选方案是在 controller 中直接检查邮箱唯一性并写入 Ent。该方案会破坏现有分层，把 HTTP 层和数据库访问耦合在一起，因此不采用。

### 请求 DTO 使用 validator tag 加 DTO 自定义校验

创建用户 DTO 使用 `json`、`validate`、`label` 和 `example` tag 表达必填、邮箱格式、长度边界和文档示例。需要默认值或跨字段规则时，通过 `SetDefaults()` 与 `Validate() error` 复用 `common/validation` 的扩展钩子。

备选方案是使用 Gin `ShouldBindJSON` 和 `binding` tag。该方案会绕过当前共享校验能力、中文错误归一化和字段名解析，因此不采用。

### 唯一性以预检查加数据库约束兜底

service 在创建前通过 repository 检查邮箱是否已存在，返回 `response.ConflictError(apperror.MsgUserAlreadyExists)`；repository 创建时仍需识别 Ent constraint 错误并映射为同一冲突错误，防止并发请求绕过预检查。

备选方案只依赖数据库唯一索引。该方案可以保证一致性，但错误文案更依赖底层异常解析，调用方体验较差，因此采用预检查加约束兜底。

### Swagger 采用服务内 docs 包和专用路由注册

在 `user-services/cmd/main.go` 放置全局 Swagger 元数据注解，在 controller 方法上放置 endpoint 注解，在 `user-services/internal/router` 中增加 Swagger 路由注册函数并 blank import 生成的 `user-services/docs` 包。访问路径采用 `/swagger/*any`，并提供 `/docs`、`/api-docs` 重定向。

备选方案是维护手写 OpenAPI YAML。手写文件容易和 Gin 路由、DTO tag 漂移，不能复用 Go 注解和 DTO 类型，因此不采用。

### Swagger 启用策略保持简单且可覆盖

Swagger UI 默认在非生产环境启用，生产环境默认关闭，并允许通过 `SWAGGER_ENABLED=true|false` 显式覆盖。该策略复用参考项目思想，但实现和说明必须与当前配置结构一致。

备选方案是新增 YAML 配置项。当前配置 loader 不做字段 required/range 校验，且本变更不需要扩大配置模型；环境变量开关足以满足本次需求，因此不新增配置字段。

### 文档响应模型必须反映真实响应

业务 API Swagger 注解使用 `common/response.Envelope{data=...}` 表达统一信封，错误返回也使用同一信封类型。健康检查如果运行时返回最小状态 JSON，则 Swagger 注解必须使用健康检查响应类型，而不是虚构为业务响应信封。

备选方案是为 Swagger 单独定义一套响应 wrapper。该方案会造成文档模型与运行时模型分叉，因此不采用。

## Risks / Trade-offs

- Swagger 生成工具版本漂移 -> 在 `user-services/go.mod` 固定 swaggo 依赖版本，并记录 `swag init` 命令。
- swaggo 对 internal 包 schema 名称生成较长 -> 通过清晰的 DTO 包名和注解保持可读，不为缩短名称牺牲目录结构。
- 创建用户并发冲突可能只在数据库写入时暴露 -> repository 同时处理唯一性约束错误，返回统一 409 冲突。
- Swagger UI 暴露内部接口信息 -> 生产默认关闭，并保留 `SWAGGER_ENABLED` 覆盖能力。
- Ent schema 如果需要调整会引入 migration -> 优先复用现有 schema；如必须修改，只通过 Ent schema、`go generate ./ent` 和 Atlas migration 生成流程完成。

## Migration Plan

1. 引入 Swagger 依赖、注解、文档路由和生成产物。
2. 新增创建用户 DTO、controller、service、repository 方法和路由注册。
3. 补齐现有查询接口与健康检查 Swagger 注解，确保注解路径与真实路由一致。
4. 运行 `swag init` 生成 `user-services/docs`，并运行 `gofmt`。
5. 在 `common/` 和 `user-services/` 分别运行 `go test ./...`。
6. 如涉及 Ent schema 变更，在 `user-services/` 运行 `go generate ./ent`、`./scripts/migrate-diff.sh <name>` 和 `./scripts/migrate-validate.sh`。

回滚时移除新增 route、Swagger docs/依赖与创建用户代码；若产生数据库 migration，按部署策略使用反向 migration 或恢复到变更前 schema。

## Open Questions

- 创建用户是否允许调用方传入 `active`，还是固定默认 `true`。默认设计为可选字段，缺省时为 `true`，与 Ent schema 保持一致。
- 用户名是否需要全局唯一。当前 Ent schema 只约束邮箱唯一，默认不增加用户名唯一性，避免引入未要求的数据模型变化。
