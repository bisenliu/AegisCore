## Why

`user-services/internal/dto` 使用技术概念命名，当前虽尚未成为兜底目录，但已经承载用户资料与认证会话的 HTTP 请求、响应和 Swagger 文档模型，并被 controller、service、validation、Swagger 注释和测试广泛引用。随着 `user-profile-query`、`user-profile-create`、`user-session-control` 和 `api-swagger-documentation` 持续扩展，继续保留全局 `dto` 包会增加业务归属不清和请求/响应模型混杂的风险。

## What Changes

- 将 `user-services/internal/dto` 中的 HTTP API 契约模型迁移到按业务能力组织的 `user-services/internal/api/user` 和 `user-services/internal/api/auth` 包。
- 在业务 API 包内使用 `request.go`、`response.go`，必要时使用 `doc.go` 区分请求、响应和 Swagger-only 文档模型。
- 更新 controller、service、validation、Swagger 注释和测试中的 imports 与类型引用，使类型归属表达为 `userapi.*` 或 `authapi.*`。
- 删除或清空不再使用的 `internal/dto` 兜底包，避免后续继续向该目录添加无明确业务归属的模型。
- 保持现有 HTTP 路由、JSON 字段、请求校验、响应信封、错误码、认证语义、Swagger 生成内容和运行时行为不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-profile-query`: 用户查询请求和响应模型迁移到用户 API 契约包后，查询接口的外部行为、公开字段和分层职责必须保持不变。
- `user-profile-create`: 用户创建请求和响应模型迁移到用户 API 契约包后，创建接口的校验、响应和冲突语义必须保持不变。
- `user-session-control`: 登录、刷新、改密和登出相关请求/响应模型迁移到认证 API 契约包后，认证会话语义和响应结构必须保持不变。
- `api-swagger-documentation`: Swagger 注释和生成文档必须跟随新的 API 契约包类型引用，并保持与运行时路由和响应契约一致。

## Impact

- 影响代码：`user-services/internal/dto/`、`user-services/internal/controller/`、`user-services/internal/service/`、`user-services/internal/validation/`、相关 `user-services/internal/**/*_test.go` 和 Swagger 注释。
- 影响文档生成：Swagger 注释中的类型引用需要从 `dto.*` 更新为新的业务 API 包类型。
- API 兼容性：不改变 HTTP 路径、方法、请求体字段、query/path 参数、响应 JSON 字段、响应信封、错误码或认证要求。
- 数据和依赖：不修改 Ent schema、Atlas migration、PostgreSQL/Redis 数据结构、配置项或 common 模块公共契约。
