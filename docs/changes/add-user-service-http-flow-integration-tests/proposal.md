# Add user-service HTTP flow integration tests

## What

为 `user-service` 增加真实 HTTP 请求流程的 integration/e2e 测试，覆盖从 `httptest` 请求进入 Gin engine，经 Fx 依赖装配、middleware、application use case、PostgreSQL/Redis adapter，到统一响应信封返回的关键路径。

重点覆盖：

- 使用 `common/testing/containers` 启动真实 PostgreSQL 和 Redis。
- 使用 Atlas migration 或等价 migration apply 流程初始化测试数据库 schema。
- 使用用户服务 Fx 测试模块组装真实 Gin engine、feature module、providers、Ent clients、JWT service、PostgreSQL 和 Redis resources。
- 通过 `httptest` 发起完整 HTTP 请求，不直接调用 controller 方法。
- 覆盖主路径：创建用户或准备用户凭据 -> 登录 -> 获取用户信息 -> 修改密码 -> 登出当前设备。
- 覆盖关键中间件行为：trace-id 透传/响应头、JWT auth、token version validation、统一错误响应。
- 将该测试显式标记为 integration/e2e，默认不影响普通 `go test ./...` 和 `make test`。

本变更只新增测试能力和必要测试 helper，不改变业务 API、数据库 schema、Redis key、迁移内容或生产 runtime wiring。

## Why

当前测试覆盖了 controller、application、infrastructure adapter、provider 和 middleware 的局部行为，但缺少一条从 HTTP 请求到真实数据库/缓存再返回 HTTP 响应的完整链路测试。这会留下几类风险：

- Fx 依赖图在局部 `ValidateApp` 中可通过，但真实 HTTP route graph 与 provider 组合仍可能在集成时缺依赖或命名不一致。
- Ent schema 与 Atlas migration 之间如果漂移，SQLite 或 mock 测试无法发现 PostgreSQL 真实 schema 问题。
- Redis session store、JWT 中间件、token version validation 和 protected route 组合如果出现配置、key、TTL 或上下文传递问题，单层测试很难捕捉。
- Gin middleware 链顺序、trace-id 注入、panic recovery、CORS、auth abort 和 response envelope 的真实行为需要 HTTP 请求级验证。

新增 integration/e2e 测试可以把最关键的用户认证流程作为一条可重复执行的“系统冒烟线”，降低后续调整 provider、middleware、migration 或 auth/session 逻辑时的回归风险。

## Scope

包括：

- 新增 `user-service/tests/e2e` 或等价测试包，用于承载 user-service HTTP flow integration 测试和测试 harness。
- 复用 `common/testing/containers.StartPostgres` 和 `StartRedis`，不复制 testcontainers 启动逻辑。
- 为测试 PostgreSQL 执行 `user-service/migrations` 下 SQL migration，或调用项目已有 Atlas migration helper/脚本的 Go 测试等价路径。
- 构造测试专用 config，指向容器 PostgreSQL/Redis、短 token TTL、测试日志输出和随机可用 HTTP listen 地址。
- 用 Fx 测试模块或 `fxtest` 组装真实依赖，并通过 `fx.Populate` 获取 `*gin.Engine` 或 `*http.Server`。
- 用 `httptest.NewRecorder` 和 `httptest.NewRequest` 直接请求 Gin engine。
- 为完整路径编写测试 helper：创建测试用户、登录、读取 access/refresh token、调用 protected user API、修改密码、登出当前设备。
- 验证响应使用 `common/contract/response.Envelope`，并断言关键字段、HTTP status、trace header 和认证失败响应。
- 使用 `AEGISCORE_TEST_CONTAINERS=1` 或更明确的 `AEGISCORE_TEST_E2E=1` 启用，未启用时跳过。
- 更新 `docs/TESTING.md`，说明 user-service HTTP flow integration 测试的启用命令、依赖和覆盖范围。

不包括：

- 不新增 OpenSpec/OPSX 工件。
- 不修改生产 HTTP route、controller、application use case、Redis key 或 JWT claim 语义。
- 不通过运行时 `client.Schema.Create(ctx)` 初始化 schema。
- 不把 user/auth 业务 fixture 放入 `common/testing`。
- 不引入真实 MQ、eventbus、outbox、外部 HTTP/gRPC client 或 RBAC 业务。
- 不要求普通 `make test` 默认启动 Docker 或真实数据库。

## Acceptance Criteria

- 存在 user-service HTTP flow integration 测试，默认未启用时稳定跳过，不影响普通 `go test ./...`。
- 启用 integration/e2e 开关且 Docker 可用时，测试会启动真实 PostgreSQL 和 Redis。
- 测试数据库 schema 通过 Atlas migration SQL 初始化，不使用 Ent runtime auto schema create。
- 测试通过 Fx 组装真实 user-service Gin route graph 和业务依赖，而不是直接实例化 controller 调方法。
- 测试覆盖登录、获取当前或指定用户信息、修改密码、登出当前设备的成功路径。
- 测试至少覆盖一个认证失败或登出后 token/session 失效路径。
- 测试断言统一响应信封、关键 token/user 字段、trace-id 响应头和受保护路由认证行为。
- `docs/TESTING.md` 记录运行命令、Docker 不可用时行为、migration 初始化策略和普通测试不依赖 Docker 的规则。
- 没有新增 `openspec/`、`docs/opsx/`、业务 schema migration 或生产 runtime 行为变更。
