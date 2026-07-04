## 背景

`user-service/tests/e2e` 中的 HTTP flow、migration harness 和测试 harness 仍使用 `t.Fatal`、`t.Fatalf` 与手写 if 拼装断言。它们覆盖完整 user-service 启动、PostgreSQL/Redis Testcontainers、Atlas SQL migration 应用、认证会话、用户资料和受保护 HTTP 响应，失败信息与 `docs/TESTING.md`、`delivery-operations` 主规格中固化的语义化断言规范不一致。

本 change 将这些历史 E2E Go 测试迁移到统一断言规范，不引入旧断言兼容层，也不改变 E2E 流程、测试数据构造、Testcontainers 前置条件、生产 HTTP API、数据库 schema 或 migration 文件。

## 变更内容

- 将 `user-service/tests/**/*_test.go` 中可由 `testify/require` 或 `testify/assert` 清晰表达的历史失败判断迁移为语义化断言。
- HTTP flow 中的 status、错误码、响应 envelope、token metadata、用户响应字段、改密与登出结果优先使用 `require.NoError`、`require.NotEmpty`、`require.Equal`、`require.Greater`、`require.Empty`、`require.ErrorContains`、`require.JSONEq`、`require.Regexp` 或等价语义化断言。
- 对完整 HTTP flow 中多个互相独立的响应字段检查，可按 `docs/TESTING.md` 使用 `testify/assert` 收集独立失败；初始化、前置条件、解码和依赖后续检查的结果继续使用 `require`。
- migration validation 和测试 harness 中的 migration 文件枚举、SQL 读取、语句拆分、逐条执行、服务根目录定位、配置写入、端口解析、JSON marshal/unmarshal、response data 解码和 PostgreSQL 打开失败使用语义化断言表达。
- 残留扫描中确实不属于 `testing.T` 失败方法的 false positive，或符合 `docs/TESTING.md` 特殊例外规则的测试控制流，必须在 `tasks.md` 中列明。
- 不新增旧 API 响应、旧 migration 行为、旧测试数据兼容断言，不新增机械 `Fail` / `Failf` / `FailNow` / `FailNowf` 替换或旧手写断言兼容 helper。

## 能力影响

### 新增能力

无。

### 修改能力

- `delivery-operations`: 明确 `user-service/tests` 的 E2E HTTP flow、migration validation 和测试 harness 必须遵循统一 Go 测试断言规范，并通过残留扫描、testify 使用扫描、目标测试和 OpenSpec 校验验收。
- `runtime-observability`: 明确 E2E harness 启动 user-service runtime、配置日志目录、构造 Gin engine、处理 response envelope 和容器前置条件时应使用语义化断言，保持运行时观测和启动语义不变。
- `auth-session-management`: 明确认证会话 E2E flow 的登录、强制改密、改密、登出和 refresh 失败响应断言应使用语义化 `require` / 必要 `assert`，不接受旧 token 字段或旧错误码兼容断言。
- `rbac-access-control`: 明确跨 feature E2E flow 中受保护用户接口的认证和授权边界响应断言应遵循统一断言规范，并保持当前中间件、授权和错误 envelope 语义不变。
- `user-identity-management`: 明确用户创建、用户详情、状态流转和公开用户字段的 E2E 响应断言应使用语义化断言，不接受旧用户响应字段或旧测试数据兼容分支。

## 影响范围

- 代码范围限定为 `user-service/tests/**/*_test.go`，当前目标文件为 `user-service/tests/e2e/http_flow_test.go`、`user-service/tests/e2e/migrations_test.go` 和 `user-service/tests/e2e/harness_test.go`。
- `user-service/go.mod` 已直接声明 `github.com/stretchr/testify`，预计无需新增依赖；若 `go mod tidy` 产生与本 change 无关的漂移不得提交。
- 不影响生产 HTTP API、OpenAPI 生成物、数据库 schema、Atlas migration、部署资产、认证会话运行时行为、RBAC 授权语义、用户资料业务语义、日志字段或 metrics/tracing 语义。
- E2E 目标测试依赖 Testcontainers 环境，运行 `go test ./user-service/tests/...` 时需要启用 `AEGISCORE_TEST_E2E=1` 或通用容器测试开关；若环境缺少容器运行时，tasks 必须记录跳过或替代验证结果。
- 验证重点包括残留失败调用扫描、`testify/require|assert` 使用扫描、`go test ./user-service/tests/...` 和 `openspec validate standardize-e2e-test-assertions-no-compat`。
