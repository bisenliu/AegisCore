# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md` 和 `docs/TESTING.md`，确认 integration/e2e 测试边界和 Docker 启用规则。
- [x] 阅读 user/auth HTTP routes、request DTO 和 response DTO，确认实际 path、method 和 JSON 字段。
- [x] 阅读 `common/testing/containers` API，确认 PostgreSQL/Redis container helper 返回的 config/DSN 可供 user-service config 使用。
- [x] 新增 `user-service/internal/integrationtest/` 测试包。
- [x] 新增 e2e 启用判断 helper，支持 `AEGISCORE_TEST_E2E=1`，并兼容 `AEGISCORE_TEST_CONTAINERS=1`。
- [x] 新增 PostgreSQL/Redis 容器启动 harness，复用 `common/testing/containers`，不复制 testcontainers 逻辑。
- [x] 新增测试配置生成 helper，把容器连接信息写入临时 YAML config。
- [x] 新增 migration apply helper，从 `user-service/migrations/*.sql` 按顺序初始化测试 PostgreSQL。
- [x] 确认 migration apply helper 不使用 `client.Schema.Create(ctx)`，不创建测试专用手写 schema。
- [x] 新增 Fx harness，复用 `bootstrap.AppModule`、`config.NewConfig` 和 `logger.NewLogger`，并通过 `fx.Populate` 获取 `*gin.Engine`。
- [x] 新增 HTTP request helper，使用 `httptest` 请求 Gin engine，并支持 JSON body、Bearer token 和 trace-id header。
- [x] 新增 response envelope decode helper，断言 HTTP status、`success`、`code`、`message` 和关键 `data` 字段。
- [x] 新增创建用户 helper，调用 `POST /api/v1/users` 准备可登录测试用户。
- [x] 新增登录 helper，调用 `POST /api/v1/auth/login` 并提取 access token、refresh token 和必要 session 字段。
- [x] 新增受保护用户查询 helper，调用 `GET /api/v1/users/:id` 并断言 JWT middleware、token version validation 和 user query 链路可用。
- [x] 新增修改密码 helper，调用实际 change-password route，并断言旧密码登录失败、新密码登录成功。
- [x] 新增登出当前设备 helper，调用实际 logout-current route，并断言登出后当前 session/token 或 refresh 行为失效。
- [x] 在主测试中覆盖 `登录 -> 获取用户信息 -> 修改密码 -> 登出` 完整路径。
- [x] 至少新增一个失败路径断言，例如无 Authorization 访问受保护路由、旧密码登录失败、登出后 refresh 失败或 token/session 失效。
- [x] 确认测试失败输出不打印完整 token、Authorization header、refresh token、Cookie、原始密码或完整请求体。
- [x] 运行 `gofmt -w` 处理新增 Go 测试文件。
- [x] 更新 `docs/TESTING.md`，记录 user-service HTTP flow integration 测试的目的、启用命令、默认跳过和真实 PostgreSQL/Redis/migration 依赖。

## Verification

- [x] 在 `user-service/` 执行 `go test ./internal/integrationtest`，确认未启用时测试 skip 且命令通过。
- [x] 在 Docker 可用环境中执行 `AEGISCORE_TEST_E2E=1 go test ./internal/integrationtest -run TestHTTPAuthUserFlow -count=1`。
- [x] 在 `user-service/` 执行 `go test ./...`，确认普通测试不要求 Docker。
- [x] 在仓库根目录执行 `make test`，确认全仓库普通测试不要求 Docker。
- [x] 如修改或复用 `common/testing`，在 `common/` 执行 `go test ./...`。
- [x] 检查 migration 初始化日志或测试断言，确认 schema 来自 `user-service/migrations`。
- [x] 检查响应断言，确认成功和失败响应均符合统一 envelope。
- [x] 检查 trace-id 断言，确认 middleware 链真实执行。
- [x] 检查 auth 断言，确认 protected route 经过 JWT middleware 和 token version validation。
- [x] 运行 `rg -n "client\\.Schema\\.Create" user-service/internal/integrationtest`，确认测试没有运行时自动建表。
- [x] 运行 `find . -maxdepth 2 \( -path './openspec' -o -path './docs/opsx' \) -print`，确认没有新增 OpenSpec/OPSX 工件。
- [x] 检查 `git diff`，确认没有业务逻辑、HTTP API、数据库 schema、migration 或 Redis key 的非预期变化。

## Review Notes

- [x] 确认测试包属于 `user-service`，没有把 user/auth 业务 fixture 放入 `common/testing`。
- [x] 确认 Fx harness 尽量复用生产 AppModule/provider，不维护一套偏离生产的测试依赖图。
- [x] 确认容器测试只有显式启用时运行，普通 `make test` 保持轻量稳定。
- [x] 确认 migration apply 失败时错误信息能定位具体 migration 文件。
- [x] 确认 helper 只在测试代码中出现，不进入生产 runtime package。
- [x] 确认文档清楚说明这类测试验证 Fx 装配、migration、中间件链和真实 PostgreSQL/Redis，而不是替代所有单元测试。
