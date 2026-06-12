# Move feature API DTO into transport HTTP

## What

将 user-service 内 feature 的 HTTP request/response DTO 从 feature-level `api/` 包迁移到对应 feature 的 `transport/http` 包，贴合最终目录结构中 HTTP DTO 归属 HTTP 边界的规则。

目标结构：

```text
user-service/internal/features/
  user/
    application/
    domain/
    infrastructure/postgres/
    transport/http/
      controller.go
      mapper.go
      request.go
      response.go
      routes.go
      validation.go
    fx.go
  auth/
    application/
    domain/
    infrastructure/postgres/
    infrastructure/redis/
    transport/http/
      controller.go
      mapper.go
      request.go
      response.go
      routes.go
      validation.go
    fx.go
```

本变更迁移：

- `user-service/internal/features/user/api/request.go` -> `user-service/internal/features/user/transport/http/request.go`。
- `user-service/internal/features/user/api/response.go` 和 Swagger doc model -> `user-service/internal/features/user/transport/http/response.go`。
- `user-service/internal/features/auth/api/request.go` -> `user-service/internal/features/auth/transport/http/request.go`。
- `user-service/internal/features/auth/api/response.go` -> `user-service/internal/features/auth/transport/http/response.go`。
- Controller、mapper、validation、validation test 和 Swagger 注解中对旧 `userapi`、`authapi` DTO 的引用。
- 当前长期文档中关于 feature `api/` DTO 目录的描述。

迁移后 HTTP DTO 直接属于 `transport/http` 包。Controller 继续将 HTTP DTO 映射为 application command/query 后调用 service；application 和 domain 仍不依赖 HTTP DTO。

## Why

当前 `api/` 包只承载 HTTP request/response DTO 和 Swagger doc model，实际消费者也集中在 `transport/http` controller、mapper、validation 和测试中。它不是跨 transport 的稳定契约，也不是 application/domain 可复用模型。

把这些 DTO 收拢到 `transport/http` 可以让边界更清晰：

- HTTP 绑定 tag、JSON tag、Swagger example 和响应展示模型都留在 HTTP 传输层。
- Feature 根目录减少一个只服务 HTTP 的包，目录结构更贴近最终分层。
- Application 层继续只暴露 command/query/result 和 ports，避免服务层误用 HTTP DTO。

这次变更只迁移 DTO 所在目录和 package 引用，不改变 HTTP API、JSON 字段、响应信封、状态码、错误码或业务流程。

## Scope

包括：

- 移动 user feature 的请求 DTO：`GetUserRequest`、`ListUsersRequest`、`CreateUserRequest`。
- 移动 user feature 的响应 DTO：`UserResponse`、`UserResponseDoc`、`UserListResponseDoc`。
- 移动 auth feature 的请求 DTO：`LoginRequest`、`RefreshTokenRequest`、`ChangePasswordRequest`。
- 移动 auth feature 的响应 DTO：`TokenResponse`、`LogoutResponse`、`ChangePasswordResponse`。
- 保持 DTO struct 字段、JSON/query/uri/validate/label/example tag 和 `SetDefaults` 行为不变。
- 更新 user/auth HTTP controller 中 request 初始化和 Swagger `@Param`、`@Success` 注解的 type references。
- 更新 user/auth HTTP mapper 返回类型和构造逻辑，使其使用同包 DTO。
- 更新 user/auth HTTP validation 函数和 validation tests，使其使用同包 DTO。
- 删除迁移后的空 `user-service/internal/features/user/api/` 和 `user-service/internal/features/auth/api/` 包。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 中当前目录结构和分层规则。
- 运行 `gofmt` 格式化受影响 Go 文件，并重新生成 Swagger 文档。

不包括：

- 不让 application 或 domain 依赖 HTTP DTO。
- 不改变 JSON tag、query tag、uri tag、validate tag、example tag 或 API 响应结构。
- 不改变 controller 到 application command/query 的映射语义。
- 不改变 application service、commands、queries、ports、result 或 domain 类型。
- 不改变 HTTP route、HTTP method、response envelope、状态码或错误码。
- 不改变 Ent schema、Ent generated code、Atlas migration、PostgreSQL/Redis adapter 或配置。
- 不新增横向 `internal/api`、`internal/dto`、`internal/shared` 或新的跨 feature DTO 包。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/features/user/transport/http/request.go` 存在并承载 user request DTO。
- `user-service/internal/features/user/transport/http/response.go` 存在并承载 user response DTO 和 Swagger doc model。
- `user-service/internal/features/auth/transport/http/request.go` 存在并承载 auth request DTO。
- `user-service/internal/features/auth/transport/http/response.go` 存在并承载 auth response DTO。
- `user-service/internal/features/user/api/` 和 `user-service/internal/features/auth/api/` 不再存在。
- 当前 Go 代码中不再导入 `github.com/aegiscore/user-service/internal/features/user/api` 或 `github.com/aegiscore/user-service/internal/features/auth/api`。
- Swagger 注解引用迁移后的 HTTP package/model 名称，`make swagger-generate` 可成功运行。
- HTTP controller、mapper、validation 和 validation tests 编译通过并继续使用同一组 DTO 字段和 tag。
- Application 和 domain 包不导入 `transport/http`，也不接收 HTTP request/response DTO。
- HTTP API 的 JSON 字段、响应 envelope、状态码和错误码无变化。
- 从仓库根目录运行 `rg -n 'features/(user|auth)/api|\buserapi\b|\bauthapi\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md` 不发现当前业务引用。
- 在 `user-service/` 下运行 `go test ./internal/features/user/transport/http ./internal/features/auth/transport/http` 通过。
- 在仓库根目录运行 `make swagger-generate` 通过。
