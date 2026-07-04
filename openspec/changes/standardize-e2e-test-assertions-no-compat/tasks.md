## 1. 基线与依赖确认

- [x] 1.1 确认 `user-service/go.mod` 已直接声明 `github.com/stretchr/testify`，本 change 不新增无关依赖或 tidy 漂移。
- [x] 1.2 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/tests --glob "*_test.go"`，记录迁移前历史失败调用和 `fmt.Errorf` 扫描 false positive 基线。
  - 迁移前命中 33 行：29 个可迁移 `testing.T` 失败调用，4 个 `splitSQLStatements` 中 `fmt.Errorf(...)` 正则 false positive。
- [x] 1.3 运行 `rg "github.com/stretchr/testify/(require|assert)" user-service/tests --glob "*_test.go"`，记录迁移前 testify 使用情况。
  - 迁移前无命中。

## 2. 测试 harness 断言迁移

- [x] 2.1 迁移 `user-service/tests/e2e/harness_test.go` 中 `writeTestConfig`、`freeTCPPort`、request body marshal、response envelope decode、response data decode 和 `openPostgres` 的错误断言，使用 `require.NoError`、`require.NotEmpty`、`require.Greater` 或等价语义化断言。
- [x] 2.2 迁移 `expectEnvelope` 的 HTTP status、`success`、应用错误码和 message 断言；多个互相独立字段可使用 `assert`，后续解码依赖的 envelope 前置条件继续使用 `require`。
- [x] 2.3 保持 `requireE2EEnabled`、Testcontainers 启用逻辑、Fx app 启停、测试配置内容、请求 header 和 `RemoteAddr` 构造不变。

## 3. migration harness 断言迁移

- [x] 3.1 迁移 `user-service/tests/e2e/migrations_test.go` 中 migration 文件 glob、空集合、SQL 文件读取、SQL 语句拆分和逐条执行失败断言，使用 `require.NoError`、`require.NotEmpty`、`require.Greater` 或等价语义化断言。
- [x] 3.2 迁移 `userServiceRoot` 的工作目录读取和 user-service 根目录定位断言，保持按父目录查找 `module github.com/aegiscore/user-service` 的逻辑不变。
- [x] 3.3 保持 `splitSQLStatements` 和 `readDollarTag` 的 SQL 解析语义、返回错误文本、注释/引号/dollar quote 处理和 migration 应用顺序不变。

## 4. HTTP flow 断言迁移

- [x] 4.1 迁移 `seedUser` 中 password service 创建、password hash、PostgreSQL seed 写入断言，保持 seed 用户字段、token version、状态和时间戳构造不变。
- [x] 4.2 迁移 `createUser`、`getUser` 的用户响应断言，使用 `require.NotEmpty`、`require.Equal` 和必要 `assert` 表达 `user_id`、`username`、`status` 等字段，不新增旧响应字段兼容断言。
- [x] 4.3 迁移普通登录和强制改密登录 token metadata 断言，使用 `require.NotEmpty`、`require.Empty`、`require.Equal`、`require.Greater` 或必要 `assert` 表达 access token、refresh token、token type 和 expires_in。
- [x] 4.4 迁移改密、旧密码登录失败、正常登录、缺失认证、登出当前会话和 refresh 失败断言，保持现有 HTTP flow 顺序、请求 body、错误码和响应 envelope 语义不变。

## 5. 例外收敛与扫描记录

- [x] 5.1 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/tests --glob "*_test.go"`，消除所有可由语义化断言表达的 `testing.T` 失败调用和机械 `Fail*` 调用。
- [x] 5.2 在本文件第 7 节记录最终剩余命中；允许的剩余项只能是 `docs/TESTING.md` 明确允许的特殊测试控制流、特殊诊断输出、测试辅助工具边界，或 `fmt.Errorf` 被验收正则误命中的 false positive。
- [x] 5.3 运行 `rg "github.com/stretchr/testify/(require|assert)" user-service/tests --glob "*_test.go"`，确认目标范围内存在迁移后的实际使用点。
  - 迁移后命中 `user-service/tests/e2e/harness_test.go`、`migrations_test.go`、`http_flow_test.go` 的 `require`/`assert` 导入。
- [x] 5.4 检查迁移后的代码未在存在更具体断言时使用 `require.True` / `require.False` 或 `assert.True` / `assert.False` 包装布尔表达式。
  - `rg "\b(require|assert)\.(True|False)\(" user-service/tests --glob "*_test.go"` 无命中。

## 6. 验证

- [x] 6.1 运行 `gofmt` 覆盖本次修改的 `user-service/tests/**/*_test.go` 文件。
- [x] 6.2 在具备 Docker 或兼容容器运行时的环境中运行 `AEGISCORE_TEST_E2E=1 go test ./user-service/tests/...`；若容器前置条件不可用，记录具体环境原因，并运行可用的替代静态验证。
  - 已运行 `AEGISCORE_TEST_E2E=1 go test ./user-service/tests/...`，Testcontainers 可启动；测试在 Fx config validation 阶段失败：`observability.metrics.path is required; observability.tracing.exporter is required`。本 change 严格要求保持测试配置内容不变，因此未改写 `writeTestConfig`。
  - 替代验证 `go test ./user-service/tests/...` 通过。
- [x] 6.3 运行 `openspec validate standardize-e2e-test-assertions-no-compat`，确认 change artifacts 通过 OpenSpec 校验。
  - 通过；命令退出前有 PostHog flush warning：`ReferenceError: Blob is not defined`。
- [x] 6.4 运行 `make user-service-architecture-lint`，确认 OpenSpec 文档语言和架构边界检查通过。
- [x] 6.5 将本次预期代码、测试和 OpenSpec 变更加到暂存区后运行 `make lint`；若环境或非本次变更阻塞，记录具体原因。
  - 未运行：用户明确要求不要 `git add`，且当前工作区已有无关 staged/unstaged 变更（`cover-rbac-cli-commands-no-compat`、`user-service/cmd`、`AGENTS.md`），不适合构造只包含本 change 的暂存区。
- [x] 6.6 保持本次预期变更已暂存，运行 `make verify`；若 E2E 容器环境、生成物 drift 或非本次变更阻塞，记录具体原因和已完成的替代验证。
  - 未运行：同 6.5，用户明确要求不要 `git add`，且当前暂存区包含非本次变更；已完成 `gofmt`、残留扫描、testify 使用扫描、`go test ./user-service/tests/...`、`openspec validate` 和 `make user-service-architecture-lint`。

## 7. 剩余例外记录

- [x] 7.1 实施完成后填写最终残留扫描结果；预期只允许 `splitSQLStatements` 中 `fmt.Errorf(...)` 被验收正则误命中的 false positive，或经代码上下文证明符合 `docs/TESTING.md` 特殊例外规则的直接 `testing.T` 失败调用。
  - 最终残留仅 4 个 `splitSQLStatements` 的 `fmt.Errorf(...)` 正则 false positive：
    - `user-service/tests/e2e/migrations_test.go:119` `fmt.Errorf("unterminated block comment")`
    - `user-service/tests/e2e/migrations_test.go:159` `fmt.Errorf("unterminated dollar quote %s", dollarTag)`
    - `user-service/tests/e2e/migrations_test.go:162` `fmt.Errorf("unterminated single-quoted string")`
    - `user-service/tests/e2e/migrations_test.go:165` `fmt.Errorf("unterminated double-quoted identifier")`
