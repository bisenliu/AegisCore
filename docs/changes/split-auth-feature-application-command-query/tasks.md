# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只调整 auth feature application 层组织。
- [x] 查看当前 `user-service/internal/features/auth/application/service.go`、`commands.go`、`result.go`、`credentials.go`、`tokens.go`、`sessions.go` 和 `ports.go`。
- [x] 查看当前 `user-service/internal/features/auth/transport/http/controller.go`、`mapper.go`、`validation.go` 和 `fx.go`。
- [x] 记录当前 auth 测试入口，至少包括 `application/service_test.go`、`application/components_test.go`、`transport/http/controller_test.go`、`infrastructure/postgres/credential_store_test.go` 和 `infrastructure/redis/session_store_test.go`。

## Package Structure

- [x] 创建 `user-service/internal/features/auth/application/command/`。
- [x] 创建 `user-service/internal/features/auth/application/validators/`。
- [x] 不创建空的 `application/query/` 包；只有出现真实 auth 读侧用例时再新增。
- [x] 保留 `user-service/internal/features/auth/application/ports.go` 作为 application 层根部消费侧 port 定义。
- [x] 创建 `user-service/internal/features/auth/application/tokenversion/`，承载 access-token middleware 和 command session lifecycle 共用的 token version 撤销校验策略。
- [x] 移除 `user-service/internal/features/auth/application/result.go`，将 result 类型放到对应 command/use case 文件。
- [x] 删除或收敛旧的单体 `application/service.go`，避免根 application service 继续同时实现全部认证流程。

## Command Use Cases

- [x] 将 `LoginCommand` 从 `application/commands.go` 迁移到 `application/command/login.go`。
- [x] 将登录业务编排迁移为 login command/use case。
- [x] 保持登录成功时 access/refresh token pair 签发和 refresh session 创建语义不变。
- [x] 保持必须改密用户只签发受限 password-change token、不创建普通 refresh session 的语义不变。
- [x] 将 `RefreshTokenCommand` 迁移到 `application/command/refresh.go`。
- [x] 将 refresh 业务编排迁移为 refresh command/use case。
- [x] 保持 refresh token rotation 开关、非轮换 session 复用和轮换失败不返回 token 的语义不变。
- [x] 将 `ChangePasswordCommand` 迁移到 `application/command/change_password.go`。
- [x] 将强制改密业务编排迁移为 change-password command/use case。
- [x] 保持改密时不额外调用 `IncrementTokenVersion`，并继续使用 credential update 返回的新 token version 刷新投影。
- [x] 将当前设备登出迁移到 `application/command/logout_current_session.go`。
- [x] 将全部设备登出迁移到 `application/command/logout_all_sessions.go`。
- [x] 保持登出从认证上下文读取 user/session 的行为和 `ErrMissingSession` 映射语义不变。
- [x] 为每个 use case 暴露窄接口或明确 service，避免 controller 依赖 catch-all `AuthService`。
- [x] 确认 command package 不导入 Gin、HTTP binder、HTTP response、Ent、Redis client 或 SQL。

## Credential Token Session Components

- [x] 将 credential verifier 迁移到 command 层或命令专用组件文件，并保持只依赖 application ports、domain 和密码 helper。
- [x] 保持 credential verifier 对登录用户不存在、密码不匹配、禁用状态和必须改密状态的错误语义不变。
- [x] 将 token issuer 迁移到 command 层或命令专用组件文件，并保持 JWT 签发、解析、subject 校验和 TTL fallback 行为不变。
- [x] 将 command 需要的 session lifecycle 迁移到 command 层或命令专用组件文件，并保持 session 创建、校验、轮换、删除和撤销语义不变。
- [x] 保持 token version validator 可由服务级 auth provider/middleware 注入。
- [x] 将共享 current token version 查询逻辑放入 `application/tokenversion`，避免复制 Redis cache miss、database fallback 和 cache backfill 语义。
- [x] 确认 credential、token、session 组件不导入 Gin、HTTP response、Ent、Redis client 或 SQL。

## Validators

- [x] 在 `application/validators` 中创建 auth application 层输入校验辅助。
- [x] 为 login command 保留空 username/password 返回 `authdomain.ErrInvalidCredentials` 的语义。
- [x] 为 refresh token command 保留空 token 或 Bearer-only token 返回 `authdomain.ErrTokenInvalid` 的语义。
- [x] 为 change-password command 保留缺失 token 和缺失新密码的现有错误语义；不要发明新的 HTTP 错误映射。
- [x] 只放置 transport-neutral 规则，避免复制 HTTP DTO binding tag 或字段标签逻辑。
- [x] 确认 validators 不导入 Gin、HTTP request/response DTO、HTTP response、Ent、Redis client 或 SQL。
- [x] 为 validators 添加聚焦单元测试。

## Root Application Contracts

- [x] 保持 `application/ports.go` 中 `UserCredentialStore`、`UserTokenVersionStore` 和 `AuthSessionStore` 由 auth application 层拥有。
- [x] 如需新增 use case 接口，优先放在 `application/command`，保持 controller 依赖明确。
- [x] 如需拆分 credential/session/token 组件接口，确保它们仍由 application/command 消费侧拥有，不由 infrastructure 定义。
- [x] 更新 result 引用，确保 command/use case 拥有自己的 transport-neutral result。
- [x] 确认不会形成 `application` 与 `application/command` 的 import cycle。

## HTTP Transport

- [x] 更新 `transport/http/controller.go` 构造函数，使 controller 依赖明确的 command/use case。
- [x] 更新 `LoginUser` handler，构造 command 层 `LoginCommand` 后调用 login use case。
- [x] 更新 `RefreshToken` handler，构造 command 层 `RefreshTokenCommand` 后调用 refresh use case。
- [x] 更新 `ChangePassword` handler，构造 command 层 `ChangePasswordCommand` 后调用 change-password use case。
- [x] 更新 `LogoutCurrentSession` handler，调用 current-session logout use case。
- [x] 更新 `LogoutAllSessions` handler，调用 all-sessions logout use case。
- [x] 保持 HTTP DTO 绑定、NormalizeLogin、NormalizeRefresh、NormalizeChangePassword 和错误响应逻辑在 `transport/http`。
- [x] 更新 `transport/http/mapper.go`，移除对 application 根部 result 类型的依赖。
- [x] 更新 controller tests 的 stub，使其反映 command/use case 拆分后的依赖。

## Infrastructure And Fx

- [x] 检查 `infrastructure/postgres/credential_store.go`，确认仅需保留 application ports import 和接口断言。
- [x] 检查 `infrastructure/redis/session_store.go`，确认仅需保留 application ports import 和接口断言。
- [x] 更新 PostgreSQL credential adapter tests 中的 application import，如 result/command 类型移动造成影响。
- [x] 更新 Redis session adapter tests 中的 application import，如 session component 移动造成影响。
- [x] 更新 `user-service/internal/features/auth/fx.go`，提供 command use case dependency holder 和各 use case constructor。
- [x] 如 controller 依赖接口，使用 `fx.As` 标注 command/use case provider。
- [x] 确认服务级 `internal/providers` 不承载 feature 业务逻辑。
- [x] 确认 `internal/providers/auth.go` 或 route wiring 对 token version validator provider 的引用仍可编译。

## Documentation

- [x] 更新 `AGENTS.md` 中 auth feature application 分层说明，加入 `command/` 和 `validators/`。
- [x] 更新 `AGENTS.md` Key Entry Points，将 auth service 入口指向新的 command/use case 文件。
- [x] 更新 `AGENTS.md` Repository Rules，说明 auth ports 仍在 `internal/features/auth/application/ports.go`，controller 映射到 command/use case。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization 和 HTTP Request Flow，说明 auth command/use case 拆分。
- [x] 更新 `docs/DEVELOPMENT.md` 中新增 auth feature 用例时的目录指引。
- [x] 如 `docs/TESTING.md` 提到旧 auth application service test 路径，同步更新。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。

## Formatting

- [x] 对受影响 Go 文件运行 `gofmt -w`。
- [x] 运行 `go test` 前确认 Go import alias 清晰，例如 `authcommand`、`authapplication`、`authdomain`。

## Verification

- [x] 运行 `test -d user-service/internal/features/auth/application/command`。
- [x] 运行 `test -d user-service/internal/features/auth/application/validators`。
- [x] 运行 `test -d user-service/internal/features/auth/application/tokenversion`。
- [x] 运行 `test ! -f user-service/internal/features/auth/application/result.go`。
- [x] 运行 `cd user-service && go test ./internal/features/auth/...`。
- [x] 如果 Fx wiring、providers 或 route registration 受影响，运行 `cd user-service && go test ./internal/providers/... ./internal/router/...`。
- [x] 如果改动触达共享 auth/security 依赖，运行 `make test-common` 或对应 common package 测试。
- [x] 运行结构扫描，确认 root application 不再保留单体 auth service：

```bash
rg -n 'type authService struct|func \(s \*authService\) Login|func \(s \*authService\) Refresh|func \(s \*authService\) ChangePassword|func \(s \*authService\) Logout' user-service/internal/features/auth/application
```

- [x] 运行依赖扫描，确认 command/validators 没有越层依赖：

```bash
rg -n 'gin-gonic|common/http/binding|common/http/response|/ent/|redis\\.|database/sql' user-service/internal/features/auth/application/command user-service/internal/features/auth/application/validators
```

- [x] 检查 `git diff -- user-service/internal/features/auth AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认没有 HTTP API、JWT 格式、Redis key、Ent schema、migration 或无关重构变更。

## Review Notes

- [x] 确认 `POST /api/v1/auth/login` 行为和响应契约无变化。
- [x] 确认 `POST /api/v1/auth/refresh` 行为和响应契约无变化。
- [x] 确认 `POST /api/v1/auth/change-password` 行为和响应契约无变化。
- [x] 确认 `POST /api/v1/auth/logout` 行为和响应契约无变化。
- [x] 确认 `POST /api/v1/auth/logout-all` 行为和响应契约无变化。
- [x] 确认 response envelope、错误码和状态码无变化。
- [x] 确认 JWT subject、claims、TTL fallback 和 Bearer 兼容行为无变化。
- [x] 确认 Redis key builder、session key 和 token version cache 语义无变化。
- [x] 确认 HTTP request/response DTO 没有移动到 application 层。
- [x] 确认 command/validators 没有导入 Gin、Ent、Redis client、SQL 或 HTTP response。
- [x] 确认 PostgreSQL 和 Redis adapter 只实现 application 层 ports。
- [x] 确认没有新增团队、角色、权限、设备列表或会话查询能力。
- [x] 确认没有修改 Ent generated code、Ent schema、Atlas migration 或部署资产。
