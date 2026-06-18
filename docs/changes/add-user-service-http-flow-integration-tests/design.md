# Design

## Overview

本变更新增 user-service HTTP flow integration 测试，用一条真实请求链路验证：

```text
httptest request
  -> Gin middleware chain
  -> router route graph
  -> transport/http controller
  -> application command/query
  -> PostgreSQL Ent adapter / Redis session adapter
  -> common response envelope
```

测试运行在 Go test 进程内，不启动真实 TCP HTTP server；Gin engine 由 Fx 测试模块装配后交给 `httptest` 调用。PostgreSQL 和 Redis 使用 `common/testing/containers` 启动真实容器。数据库 schema 通过 `user-service/migrations` 的 SQL migration 初始化，避免使用运行时 `client.Schema.Create(ctx)`。

## Test Package Layout

建议新增目录：

```text
user-service/tests/e2e/
  http_flow_test.go
  harness_test.go
  migrations_test.go
```

职责：

- `http_flow_test.go`：声明端到端测试用例和断言。
- `harness_test.go`：启动容器、构造 config、组装 Fx app、暴露 HTTP request helper。
- `migrations_test.go`：读取并按顺序执行 `user-service/migrations/*.sql`，必要时校验 `atlas.sum` 存在。

如果实现时希望减少文件数量，也可以先合并到一个 `_test.go` 文件；但测试 harness、migration apply 和业务流程 helper 应保持清晰分段。

## Enablement

测试默认跳过，避免普通单元测试依赖 Docker：

```go
if os.Getenv("AEGISCORE_TEST_E2E") != "1" && os.Getenv("AEGISCORE_TEST_CONTAINERS") != "1" {
    t.Skip("set AEGISCORE_TEST_E2E=1 to run user-service HTTP flow integration tests")
}
```

推荐命令：

```bash
cd user-service
AEGISCORE_TEST_E2E=1 go test ./tests/e2e -run TestHTTPAuthUserFlow -count=1
```

也可以支持已有容器开关：

```bash
AEGISCORE_TEST_CONTAINERS=1 go test ./tests/e2e -count=1
```

当开关已启用但 Docker、镜像拉取、端口映射、PostgreSQL ping、Redis ping 或 migration apply 失败时，测试应失败并输出明确错误，而不是跳过。

## Fx Harness

测试 harness 应尽量复用生产模块：

- `bootstrap.AppModule`
- `providers.Module`
- `authfeature.Module`
- `userfeature.Module`
- `common/validation.Module`
- `common/runtime/timezone.Module`
- `config.NewConfig`
- `logger.NewLogger`

实现可以选择两种形态：

1. 调用 `bootstrap.NewApp(configPath)` 并通过配置监听 `127.0.0.1:0`，再用真实 HTTP client 请求实际 server。
2. 构造 `fxtest.New`，复用 `bootstrap.AppModule`，并用 `fx.Populate(&engine)` 获取 `*gin.Engine` 后走 `httptest`。

优先选择第二种，原因是它验证 Fx 依赖装配和 route registration，同时测试更快、更稳定，避免端口监听与 server lifecycle 带来的额外异步复杂度。

示意：

```go
var engine *gin.Engine
app := fxtest.New(t,
    fx.Supply(config.ConfigPath(configPath)),
    fx.Provide(config.NewConfig, logger.NewLogger),
    bootstrap.AppModule,
    fx.Populate(&engine),
)
app.RequireStart()
t.Cleanup(app.RequireStop)
```

如果 `bootstrap.AppModule` 当前会强制实例化 `*http.Server` 并注册 lifecycle hook，这是可以接受的；测试 config 使用 `host: 127.0.0.1` 和 `port: 0`，避免固定端口冲突。HTTP 请求仍直接打到 `engine.ServeHTTP`。

## Test Config

harness 生成临时 YAML config，写入 `t.TempDir()`：

- `app.name`: `aegiscore-user-services-test`
- `app.environment`: `test`
- `http.host`: `127.0.0.1`
- `http.port`: `0`
- `http.shutdown_timeout`: 短超时
- `log.console`: false 或测试可控输出
- `auth.jwt_secret`: 测试专用强随机或固定 secret
- `auth.access_token_ttl`: 足够测试完成，例如 5 分钟
- `auth.refresh_token_ttl`: 足够测试完成，例如 30 分钟
- `auth.password_change_token_ttl`: 足够测试完成
- `redis.cache_redis`: 指向 Redis container config
- `postgres.user_db`: 指向 PostgreSQL container config
- `postgres.common_db`: 可指向同一 PostgreSQL container，或单独数据库；优先同容器不同 database/schema 仅在 migration 支持时使用

测试配置不得复用开发或生产配置中的真实 secret、真实 DSN 或固定外部地址。

## Database Migration Strategy

测试必须从 `user-service/migrations` 初始化 schema：

- 按 migration 文件名顺序读取 `*.sql`。
- 跳过 `atlas.sum` 和 `atlas.hcl`。
- 解析 Atlas SQL 文件中的普通 SQL statement，并忽略注释。
- 使用 `database/sql` 连接 test PostgreSQL 执行 SQL。
- 如果 migration 中存在 Atlas directive comment，不应把 comment 当作 SQL 执行。

为了降低手写 SQL splitter 风险，可以优先采用 PostgreSQL driver 可接受的整文件执行方式；如果某些文件包含多个 statement 且 driver 不支持一次执行，则实现一个仅服务 migration test 的小 splitter，必须正确处理分号、单引号、双引号、dollar-quoted string 和 SQL comment。若 splitter 复杂度过高，改为在测试中调用 Atlas CLI 前应先确认仓库开发环境对 Atlas CLI 的要求，并在文档中说明。

禁止使用：

- `client.Schema.Create(ctx)`
- 修改 Ent generated code
- 临时跳过 migration 直接建表的测试专用 schema

## HTTP Flow

主测试建议命名：

```go
func TestHTTPAuthUserFlow(t *testing.T)
```

流程：

1. 启动 PostgreSQL/Redis containers。
2. 应用 migration。
3. 启动 Fx app 并获取 Gin engine。
4. 通过测试数据库 seed 一个最小 bootstrap normal user，仅用于拿到首个受保护路由调用凭据。
5. `POST /api/v1/auth/login` 使用 bootstrap 用户名密码登录，保存 access token、refresh token 和 session 信息。
6. `POST /api/v1/users` 带 bootstrap Bearer access token 创建 must-change-password 测试用户，保存 `user_id`、`username` 和初始 password。
7. `GET /api/v1/users/:id` 带 Bearer access token 获取用户信息，断言返回用户与创建用户一致。
8. `POST /api/v1/auth/login` 使用 must-change-password 用户初始密码登录，断言返回 password-change token 且不返回 refresh token。
9. `POST /api/v1/auth/change-password` 带 password-change token 修改密码。
10. 使用旧密码登录应失败，使用新密码登录应成功。
11. `POST /api/v1/auth/logout` 带新 access token 登出当前设备。
12. 再次 refresh 当前 session，应返回认证失败或 token/session 无效响应。

实际 path 和 JSON 字段以 `features/*/transport/http/routes.go`、`request.go` 和 `response.go` 为准，不在测试里发明新 API。

## Assertions

每个 HTTP helper 应同时断言：

- HTTP status code。
- response JSON 可解码为 `common/contract/response.Envelope` 或当前 response helper 的 envelope 类型。
- `success`、`code`、`message` 与成功/失败预期一致。
- 成功响应 `data` 中的关键字段存在且类型正确。
- 请求进入 handler 后存在有效 OTel span context；HTTP trace 传播使用 W3C `traceparent` / `tracestate`，测试不依赖自定义 trace 响应头。
- 受保护路由缺少 Authorization、token 无效、登出后 session 无效等至少一个失败路径返回认证错误信封。

Token 断言不应打印或记录完整 token。测试失败信息也不要输出 Authorization header、refresh token 或原始密码。Bootstrap seed 只允许写入测试数据库以建立第一个可认证用户，不得绕过后续 HTTP flow 的核心行为。

## Test Helpers

建议 helper：

- `newHTTPFlowHarness(t) *httpFlowHarness`
- `(*httpFlowHarness).request(method, path string, body any, token string) *httptest.ResponseRecorder`
- `decodeEnvelope(t, recorder) envelope`
- `createUser(t, h, input) createdUser`
- `login(t, h, username, password) tokenPair`
- `changePassword(t, h, token, oldPassword, newPassword)`
- `logoutCurrent(t, h, token)`
- `applyMigrations(ctx, t, dsn string)`

业务 fixture 保持在 `user-service/tests/e2e` 测试包内，不进入 `common/testing/fixtures`。通用用户名、邮箱、UUID 等基础值可以复用 `common/testing/fixtures`。

## Route And DTO Discovery

实现前应读取：

- `user-service/internal/features/user/transport/http/routes.go`
- `user-service/internal/features/user/transport/http/request.go`
- `user-service/internal/features/user/transport/http/response.go`
- `user-service/internal/features/auth/transport/http/routes.go`
- `user-service/internal/features/auth/transport/http/request.go`
- `user-service/internal/features/auth/transport/http/response.go`

测试请求体必须使用现有 DTO 字段。不要新增 test-only controller、test-only route 或 production-only DTO 字段。

## Documentation Updates

更新 `docs/TESTING.md`：

- 增加 user-service HTTP flow integration 测试章节。
- 给出启用命令。
- 说明该测试使用真实 PostgreSQL/Redis 和 migration。
- 说明默认跳过，不纳入普通 `make test` 的 Docker 要求。
- 说明测试覆盖 Fx graph、middleware chain、migration、PostgreSQL/Redis adapter 和 response envelope。

如测试目录成为长期入口，也可在 `AGENTS.md` 的测试说明或 Key Entry Points 中补充，但本变更不强制修改结构规则。

## Risks And Mitigations

Risk: e2e 测试变慢或在没有 Docker 的环境中失败。

Mitigation: 默认跳过，只有显式环境变量启用后才启动容器；启用后 Docker 不可用则明确失败。

Risk: 测试为了初始化数据库绕过 migration。

Mitigation: harness 只从 `user-service/migrations` 执行 SQL；禁止 `client.Schema.Create(ctx)`。

Risk: Fx harness 为测试重建一套不同于生产的依赖图。

Mitigation: 复用 `bootstrap.AppModule` 和生产 providers，只通过测试 config 替换外部资源地址。

Risk: 测试泄漏 token、password 或 Authorization header 到日志。

Mitigation: helper 的失败消息只输出状态码、响应 code/message 和脱敏字段；不打印完整请求体或 token。

Risk: migration SQL splitter 出错，导致测试本身脆弱。

Mitigation: 优先整文件执行；需要 splitter 时覆盖注释、quote 和 dollar quote；或者明确依赖 Atlas CLI 并文档化。

## Verification Strategy

- 未启用环境变量：
  - `cd user-service && go test ./tests/e2e`
  - 期望测试 skip 且命令通过。
- 启用 Docker integration：
  - `cd user-service && AEGISCORE_TEST_E2E=1 go test ./tests/e2e -run TestHTTPAuthUserFlow -count=1`
  - 期望启动 PostgreSQL/Redis、应用 migration、完整 HTTP flow 通过。
- 回归范围：
  - `make test` 仍不要求 Docker。
  - `cd common && go test ./...`
  - `cd user-service && go test ./...`
- 文档和结构检查：
  - `rg -n "client\\.Schema\\.Create|openspec|docs/opsx" user-service/tests/e2e docs/changes/add-user-service-http-flow-integration-tests docs/TESTING.md`
  - 确认没有新增 OpenSpec/OPSX 工件，没有在测试里使用 Ent runtime auto migration。
