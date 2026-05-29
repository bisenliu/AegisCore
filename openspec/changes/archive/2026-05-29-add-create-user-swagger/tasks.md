## 1. Swagger 基础集成

- [x] 1.1 在 `user-services/go.mod` 引入并固定 `github.com/swaggo/swag`、`github.com/swaggo/gin-swagger`、`github.com/swaggo/files` 相关依赖。
- [x] 1.2 在 `user-services/cmd/main.go` 添加 AegisCore 用户服务 Swagger 全局元数据注解，确保 BasePath 为 `/api/v1` 且不声明未实现的认证必需项。
- [x] 1.3 在 `user-services/internal/router` 新增 Swagger 路由注册实现，注册 `/swagger/*any`、`/docs`、`/api-docs`，并支持生产默认关闭与 `SWAGGER_ENABLED` 覆盖。
- [x] 1.4 将 Swagger 路由注册接入现有 `RegisterRoutes`，保持现有中间件顺序和健康检查、用户 API 路由行为不变。
- [x] 1.5 补充 Swagger 生成命令说明或脚本，确保在 `user-services` 模块内可执行并生成 `docs` 产物。

## 2. 创建用户 API 实现

- [x] 2.1 在 `user-services/internal/dto` 新增创建用户请求 DTO，包含 `json`、`validate`、`label`、`example` tag，并覆盖必填、邮箱格式、名称长度、邮箱长度和 `active` 默认值。
- [x] 2.2 在 `user-services/internal/apperror` 增加用户已存在等创建用户业务错误文案，继续复用 `common/response` 错误构造函数。
- [x] 2.3 扩展 `UserRepository`，新增按邮箱存在性检查和创建用户方法，使用具名 Ent client `user_db` 并处理 Ent not found、唯一约束冲突和非预期错误。
- [x] 2.4 扩展 `UserService`，新增 `CreateUser`，执行邮箱规范化、业务校验、唯一性预检查、创建调用和响应 DTO 映射。
- [x] 2.5 扩展 `UserController`，新增 `Create` handler，使用 `common/validation.JSONBinder` 绑定请求体，成功调用 `response.Created`，失败调用 `response.Fail`。
- [x] 2.6 在 `user-services/internal/router/router.go` 注册 `POST /api/v1/users` 到 `UserController.Create`，不改变 `GET /api/v1/users/:id` 路由。

## 3. Swagger 注解与文档模型

- [x] 3.1 为创建用户 handler 添加 Swagger 注解，覆盖标签、摘要、说明、JSON 请求体、HTTP 201 成功响应、400、409、500 失败响应。
- [x] 3.2 为现有查询用户 handler 添加或统一 Swagger 注解，覆盖路径参数 `id`、HTTP 200、400、404、500 响应，并确保 `@Router /users/{id} [get]` 与真实路由一致。
- [x] 3.3 为健康检查 endpoint 添加与真实最小健康状态 JSON 一致的 Swagger 注解，不将其错误描述为业务响应信封。
- [x] 3.4 确认业务 API Swagger 响应类型复用 `common/response.Envelope` 语义，成功响应 data 指向用户 DTO，失败响应描述统一信封和业务码。
- [x] 3.5 运行 Swagger 生成命令，提交 `user-services/docs/docs.go`、`swagger.json`、`swagger.yaml`，确认生成路径、方法、状态码与 Gin 路由一致。

## 4. 数据模型与迁移检查

- [x] 4.1 复核现有 Ent `User` schema 是否已满足创建用户字段约束、邮箱唯一性和默认 `active=true`，优先避免 schema 变更。
- [x] 4.2 如必须调整 Ent schema，只修改 `user-services/ent/schema/`，在 `user-services` 模块运行 `go generate ./ent`，不得手写 `user-services/ent/` 生成代码。
- [x] 4.3 如产生 schema 变更，运行 `./scripts/migrate-diff.sh <name>` 生成 Atlas migration，审查 SQL，必要时运行 `atlas migrate hash --dir file://migrations`。
- [x] 4.4 如产生 migration，运行 `./scripts/migrate-validate.sh` 校验 migration 目录。

## 5. 测试与验证

- [x] 5.1 为创建用户 controller 添加测试，覆盖成功创建、空请求体、字段校验失败、邮箱已存在和 service 非预期错误。
- [x] 5.2 为创建用户 service/repository 添加测试或可行的集成验证，覆盖邮箱唯一性预检查、Ent 唯一约束冲突映射和普通错误包装。
- [x] 5.3 为 Swagger 路由注册添加测试或验证，覆盖启用环境访问 `/swagger/index.html`、`/docs`、`/api-docs`，以及生产默认关闭和 `SWAGGER_ENABLED` 覆盖。
- [x] 5.4 运行 `gofmt` 格式化所有新增或修改的 Go 文件。
- [x] 5.5 在 `common/` 运行 `go test ./...`。
- [x] 5.6 在 `user-services/` 运行 `go test ./...`。
- [x] 5.7 启动服务并手动验证 `GET /api/v1/users/:id` 保持兼容、`POST /api/v1/users` 返回统一响应、Swagger UI 可访问且文档内容正确。
