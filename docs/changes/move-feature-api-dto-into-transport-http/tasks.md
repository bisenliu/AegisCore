# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只迁移 HTTP DTO 归属。
- [x] 将 `user-service/internal/features/user/api/request.go` 移动到 `user-service/internal/features/user/transport/http/request.go`。
- [x] 将移动后的 user request DTO package 从 `userapi` 改为 `userhttp`，保持 `GetUserRequest`、`ListUsersRequest`、`CreateUserRequest` 和 `SetDefaults` 语义不变。
- [x] 将 `user-service/internal/features/user/api/response.go` 移动到 `user-service/internal/features/user/transport/http/response.go`。
- [x] 将 `user-service/internal/features/user/api/doc.go` 中的 `UserResponseDoc` 和 `UserListResponseDoc` 合并到 user `transport/http/response.go`。
- [x] 将移动后的 user response DTO package 从 `userapi` 改为 `userhttp`，保持所有字段、JSON tag 和 Swagger example 不变。
- [x] 将 `user-service/internal/features/auth/api/request.go` 移动到 `user-service/internal/features/auth/transport/http/request.go`。
- [x] 将移动后的 auth request DTO package 从 `authapi` 改为 `authhttp`，保持 `LoginRequest`、`RefreshTokenRequest` 和 `ChangePasswordRequest` 字段/tag 不变。
- [x] 将 `user-service/internal/features/auth/api/response.go` 移动到 `user-service/internal/features/auth/transport/http/response.go`。
- [x] 将移动后的 auth response DTO package 从 `authapi` 改为 `authhttp`，保持所有字段、JSON tag 和 Swagger example 不变。
- [x] 更新 user HTTP controller，移除 `userapi` import，并直接使用 `ListUsersRequest`、`CreateUserRequest`、`GetUserRequest`。
- [x] 更新 user HTTP mapper，移除 `userapi` import，并直接返回 `UserResponse`、`pagination.PaginatedData[UserResponse]`。
- [x] 更新 user HTTP validation 和 validation tests，移除 `userapi` import，并直接使用 moved request DTO。
- [x] 更新 user HTTP controller Swagger 注解，把旧 `userapi.*` model 引用改为迁移后的 HTTP DTO/model 引用。
- [x] 更新 auth HTTP controller，移除 `authapi` import，并直接使用 `ChangePasswordRequest`、`LoginRequest`、`RefreshTokenRequest`。
- [x] 更新 auth HTTP mapper，移除 `authapi` import，并直接返回 `TokenResponse`、`ChangePasswordResponse`、`LogoutResponse`。
- [x] 更新 auth HTTP validation 和 validation tests，移除 `authapi` import，并直接使用 moved request DTO。
- [x] 更新 auth HTTP controller Swagger 注解，把旧 `authapi.*` model 引用改为迁移后的 HTTP DTO/model 引用。
- [x] 删除迁移后的 `user-service/internal/features/user/api/` 空目录。
- [x] 删除迁移后的 `user-service/internal/features/auth/api/` 空目录。
- [x] 运行 `gofmt -w` 格式化所有受影响 Go 文件。

## Documentation

- [x] 更新 `AGENTS.md` Repository Shape，使 user/auth feature 分层不再列出 `api/`，并说明 HTTP DTO 位于 `transport/http/request.go`、`response.go`。
- [x] 更新 `AGENTS.md` Repository Rules，使 HTTP 请求 DTO 清洗、绑定后的规范化、response mapper 和 Swagger model 归属 `transport/http`。
- [x] 更新 `AGENTS.md` Dependency Rules，移除 `transport/http` 对 sibling `api` 包的依赖描述。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization 表格，去掉 `api/` 并把 HTTP DTO 责任写入 `transport/http/`。
- [x] 更新 `docs/ARCHITECTURE.md` Dependency Rules 和 controller mapping 说明，强调 application/domain 不依赖 HTTP DTO。
- [x] 更新 `docs/DEVELOPMENT.md` Coding Conventions 和 Adding Features，使服务内 feature 分层使用 `application/domain/transport/http/infrastructure/*/fx.go`，HTTP DTO 放在 transport。
- [x] 确认文档仍声明不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `test ! -d user-service/internal/features/user/api`。
- [x] 运行 `test ! -d user-service/internal/features/auth/api`。
- [x] 运行 `test -f user-service/internal/features/user/transport/http/request.go`。
- [x] 运行 `test -f user-service/internal/features/user/transport/http/response.go`。
- [x] 运行 `test -f user-service/internal/features/auth/transport/http/request.go`。
- [x] 运行 `test -f user-service/internal/features/auth/transport/http/response.go`。
- [x] 运行 `rg -n 'features/(user|auth)/api|\buserapi\b|\bauthapi\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认没有当前业务引用。
- [x] 在 `user-service/` 运行 `go test ./internal/features/user/transport/http ./internal/features/auth/transport/http`。
- [x] 从仓库根目录运行 `make swagger-generate`。
- [x] 检查 Swagger generated docs diff，确认 JSON 字段、schema 名称变化之外的 API 结构无变化。
- [x] 检查 `git diff -- user-service/internal/features AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/changes/move-feature-api-dto-into-transport-http`，确认除 DTO 目录、package/import、Swagger type references 和文档规则外没有业务逻辑变更。

## Review Notes

- [x] 确认没有让 application 或 domain 导入 `transport/http`。
- [x] 确认没有让 application service 接收 HTTP request/response DTO。
- [x] 确认 DTO 字段、JSON/query/uri/validate/label/example tag 和 `SetDefaults` 行为无变化。
- [x] 确认 controller 到 command/query 的映射语义无变化。
- [x] 确认 HTTP API、响应 envelope、错误码和状态码无变化。
- [x] 确认 Ent schema、generated code、migration 无变化。
- [x] 确认 Redis key、PostgreSQL query、JWT、session 和 token version 语义无变化。
- [x] 确认没有新增横向 `internal/api`、`internal/dto`、`internal/shared` 或新的跨 feature DTO 包。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
