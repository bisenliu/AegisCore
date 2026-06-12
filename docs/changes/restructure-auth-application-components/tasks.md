# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只调整 auth feature application 层组织。
- [x] 查看当前 `user-service/internal/features/auth/application/command/` 下的 use case 和支撑组件文件。
- [x] 查看当前 `user-service/internal/features/auth/application/ports.go`、`validators/`、`domain/`、`transport/http/`、`infrastructure/postgres/`、`infrastructure/redis/` 和 `fx.go`。
- [x] 记录当前 auth 测试入口，至少包括 command use case tests、component tests、transport/http controller tests、PostgreSQL adapter tests 和 Redis adapter tests。

## Package Structure

- [x] 创建 `user-service/internal/features/auth/application/authctx/`。
- [x] 创建 `user-service/internal/features/auth/application/credentials/`。
- [x] 创建 `user-service/internal/features/auth/application/sessions/`。
- [x] 创建 `user-service/internal/features/auth/application/tokens/`。
- [x] 保持 `user-service/internal/features/auth/application/command/` 为扁平 use case 入口。
- [x] 保留 `user-service/internal/features/auth/application/ports.go` 作为 application 层根部消费侧 port 定义。
- [x] 不创建 `application/common` 包。
- [x] 不创建 `command/login`、`command/refresh`、`command/password` 等一用例一 package。
- [x] 如果没有真实 auth query，用 README 表达 `application/query` 边界，不新增空 query handler/service/DTO。

## Auth Context

- [x] 将 command 内 authenticated session context helper 迁移到 `application/authctx/session.go`。
- [x] 保持从 `common/security/auth` context 读取 user ID 和 session ID 的行为。
- [x] 保持缺失或格式非法时返回 `authdomain.ErrMissingSession` 的语义。
- [x] 为 authctx 添加聚焦单元测试。
- [x] 确认 authctx 不导入 Gin、HTTP response、Ent、Redis client 或 SQL。

## Credentials Component

- [x] 将 credential verifier 迁移到 `application/credentials/verifier.go`。
- [x] 保持登录凭据校验、密码 hash verify、必须改密用户放行和禁用状态拒绝语义不变。
- [x] 保持登录用户不存在和密码不匹配映射为 `authdomain.ErrInvalidCredentials`。
- [x] 保持强制改密凭据更新、密码 hash、状态恢复和 token version 返回语义不变。
- [x] 保持 user-not-found 和 repository unexpected error 映射语义不变。
- [x] 为 credentials verifier 添加或迁移聚焦单元测试。
- [x] 确认 credentials 不导入 Gin、HTTP response、Ent、Redis client、SQL 或 JWT service。

## Tokens Component

- [x] 将 token issuer 迁移到 `application/tokens/issuer.go`。
- [x] 将 `TokenResult` 迁移到 `application/tokens/result.go`。
- [x] 保持 access token、refresh token 和 password-change token 签发语义不变。
- [x] 保持 `defaultAccessTokenTTL` 和 `defaultRefreshTokenTTL` fallback 行为不变。
- [x] 保持 refresh token 和 password-change token Bearer 前缀兼容解析不变。
- [x] 保持 refresh token subject 和 user ID shape 校验不变。
- [x] 保持 invalid token 映射为 `authdomain.ErrTokenInvalid`。
- [x] 为 tokens issuer 添加或迁移聚焦单元测试。
- [x] 确认 tokens 不导入 Gin、HTTP response、Ent、Redis client 或 SQL。

## Sessions Component

- [x] 将 session lifecycle 迁移到 `application/sessions/lifecycle.go`。
- [x] 将全部撤销和 token version projection 相关逻辑拆到 `application/sessions/revocation.go`，或保留在 lifecycle 文件中但保持职责清晰。
- [x] 保持 refresh session 创建、TTL、每用户活跃 session 上限传递语义不变。
- [x] 保持 password-change claims token version 校验语义不变。
- [x] 保持 refresh session 存在性、claim/session 一致性和当前 token version 校验语义不变。
- [x] 保持 refresh token rotation 的 rejected-session 错误映射为 `authdomain.ErrTokenInvalid`。
- [x] 保持当前 session 删除和全部 session 撤销语义不变。
- [x] 保持 token version cache/database fallback 与 cache backfill 语义不变。
- [x] 为 sessions lifecycle/revocation 添加或迁移聚焦单元测试。
- [x] 确认 sessions 不导入 Gin、HTTP response、Ent、Redis client、SQL、JWT service 或 password hash helper。

## Command Use Cases

- [x] 更新 `application/command/login.go`，通过 credentials、tokens 和 sessions components 完成登录编排。
- [x] 更新 `application/command/refresh_token.go`，通过 tokens 和 sessions components 完成 refresh 编排。
- [x] 更新 `application/command/change_password.go`，通过 tokens、credentials 和 sessions components 完成强制改密编排。
- [x] 更新 `application/command/logout_current_session.go`，通过 authctx 和 sessions components 完成当前设备登出。
- [x] 更新 `application/command/logout_all_sessions.go`，通过 authctx 和 sessions components 完成全部设备登出。
- [x] 如果保留 `dependencies.go`，让它只组合 component 依赖，不再承载 component 实现。
- [x] 保持 `LoginCommand`、`RefreshTokenCommand`、`ChangePasswordCommand` 与对应 use case 同文件或相邻文件。
- [x] 保持 `ChangePasswordResult`、`LogoutResult` 在 command 层对应业务语义文件中，或迁入明确 result 文件。
- [x] 确认 command package 不导入 Gin、HTTP binder、HTTP response、Ent、Redis client 或 SQL。
- [x] 更新 command use case tests，覆盖登录、刷新、强制改密和登出关键流程。

## HTTP Transport

- [x] 更新 `transport/http/controller.go` 对 command use case 和 token result 类型的引用。
- [x] 更新 `transport/http/mapper.go`，使 token response mapper 使用 `application/tokens.TokenResult` 或新的 command result 类型。
- [x] 保持 HTTP DTO、binding、NormalizeLogin、NormalizeRefresh、NormalizeChangePassword 和错误响应逻辑在 `transport/http`。
- [x] 确认 controller 仍将 HTTP DTO 映射为 application command DTO 后调用 use case。
- [x] 更新 controller tests 的 stubs 和 import path。
- [x] 确认 request/response JSON、HTTP status、response envelope 和错误映射无变化。

## Infrastructure And Fx

- [x] 检查 `infrastructure/postgres/credential_store.go`，确认仅需保留 application ports import 和接口断言。
- [x] 检查 `infrastructure/redis/session_store.go`，确认仅需保留 application ports import 和接口断言。
- [x] 更新 `user-service/internal/features/auth/fx.go`，提供 credentials、tokens、sessions component constructor 和 command use case constructor。
- [x] 如 controller 依赖接口，使用 `fx.As` 标注 command/use case provider。
- [x] 确认服务级 `internal/providers` 不承载 feature 业务逻辑。
- [x] 确认 PostgreSQL 和 Redis adapter 不导入 command、credentials、sessions 或 tokens component 包。

## Documentation

- [x] 更新 `AGENTS.md` 中 auth feature application 分层说明，加入 `authctx`、`credentials`、`sessions`、`tokens` 和扁平 `command` 入口。
- [x] 更新 `AGENTS.md` Key Entry Points，将 auth command 和 component 入口指向新文件。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization、HTTP Request Flow 和 dependency rules。
- [x] 更新 `docs/DEVELOPMENT.md` 中新增 auth application use case 或 component 时的目录指引。
- [x] 如 `docs/TESTING.md` 提到旧 auth application test 路径，同步更新。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。

## Formatting

- [x] 对受影响 Go 文件运行 `gofmt -w`。
- [x] 运行 `go test` 前确认 Go import alias 清晰，例如 `authcommand`、`authtokens`、`authsessions`、`authcredentials`、`authdomain`。

## Verification

- [x] 运行 `test -d user-service/internal/features/auth/application/authctx`。
- [x] 运行 `test -d user-service/internal/features/auth/application/credentials`。
- [x] 运行 `test -d user-service/internal/features/auth/application/sessions`。
- [x] 运行 `test -d user-service/internal/features/auth/application/tokens`。
- [x] 运行结构扫描，确认没有新增 `application/common`：

```bash
test ! -d user-service/internal/features/auth/application/common
```

- [x] 运行结构扫描，确认没有新增一用例一 command 子包：

```bash
find user-service/internal/features/auth/application/command -mindepth 1 -maxdepth 1 -type d
```

- [x] 运行依赖扫描，确认 application component 和 command 没有越层依赖：

```bash
rg -n 'gin-gonic|common/http/binding|common/http/response|/ent/|redis\\.|database/sql' user-service/internal/features/auth/application/authctx user-service/internal/features/auth/application/command user-service/internal/features/auth/application/credentials user-service/internal/features/auth/application/sessions user-service/internal/features/auth/application/tokens user-service/internal/features/auth/application/validators
```

- [x] 运行 `cd user-service && go test ./internal/features/auth/...`。
- [x] 如果 Fx wiring、providers 或 route registration 受影响，运行 `cd user-service && go test ./internal/providers/... ./internal/router/...`。
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
- [x] 确认 domain 没有导入 application、logger、config、Gin、Ent、Redis、JWT 或 password hash helper。
- [x] 确认 PostgreSQL 和 Redis adapter 只实现 application 层 ports。
- [x] 确认没有新增团队、角色、权限、MFA、OAuth、设备列表或会话查询能力。
- [x] 确认没有修改 Ent generated code、Ent schema、Atlas migration 或部署资产。
