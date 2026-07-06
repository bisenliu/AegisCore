## 1. 基线扫描

- [x] 1.1 扫描 `common/http/**/*_test.go` 中的历史失败断言：`rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/http --glob '*_test.go'`，记录需要迁移的文件和语句。
- [x] 1.2 扫描现有 `testify` 使用点：`rg "github.com/stretchr/testify/(require|assert)" common/http --glob '*_test.go'`，确认可复用的断言风格和 import 位置。
- [x] 1.3 确认本次只修改 `common/http/**/*_test.go` 和本 change artifacts，不修改 `common/http` 生产代码、`user-service` HTTP transport 测试、OpenAPI 生成物、数据库 migration 或部署资产。

## 2. common/http 测试迁移

- [x] 2.1 迁移 `common/http/binding/**/*_test.go` 的绑定成功、绑定失败、校验错误和错误响应断言，优先使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.Contains` 和 `require.JSONEq`。
- [x] 2.2 迁移 `common/http/response/**/*_test.go` 的 HTTP status、header、JSON envelope、错误详情和分页断言，避免旧 envelope 兼容断言或双写断言。
- [x] 2.3 迁移 `common/http/middleware/**/*_test.go` 的认证上下文、授权、CORS、logging、metrics、recovery、request ID 和 tracing 相关断言；前置条件使用 `require`，多个独立响应字段需要聚合失败时才使用 `assert`。
- [x] 2.4 迁移 `common/http/openapi/**/*_test.go` 的 JSON、YAML、HTML、转换和渲染输出断言，JSON body 优先使用 `require.JSONEq` 或结构化解析后的语义化断言。
- [x] 2.5 迁移 `common/http/pprof/**/*_test.go` 的 route 注册、HTTP status、header 和响应断言，不新增旧 pprof route 兼容断言或测试专用生产分支。
- [x] 2.6 对仍需保留的 `t.Fatal*`、`t.Error*` 或 `Fail*` 命中逐项确认原因；仅保留符合 `docs/TESTING.md` 特殊例外规则的自定义测试控制流、特殊诊断输出或测试辅助工具场景，并在本任务文件补充例外清单。

## 3. 验证与收尾

- [x] 3.1 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/http --glob '*_test.go'`，确认剩余命中均已在 2.6 的例外清单列明；如果无剩余命中，记录为“无”。
- [x] 3.2 运行 `rg "github.com/stretchr/testify/(require|assert)" common/http --glob '*_test.go'`，确认迁移后存在实际 `require` / 必要 `assert` 使用点。
- [x] 3.3 运行 `go test ./common/http/...`，确认 `common/http` 测试通过。
- [x] 3.4 运行 `openspec validate standardize-common-http-test-assertions-no-compat`，确认 change spec 有效。
- [x] 3.5 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区，再运行 `make lint` 和 `make verify`；如验证失败，修复后重新执行，不得在失败或未运行时标记本任务完成。

## 4. 例外清单

- [x] 4.1 在完成 3.1 后更新本节：列明每个保留的 `t.Fatal*`、`t.Error*` 或 `Fail*` 命中的文件、原因和对应 `docs/TESTING.md` 特殊例外；若无保留命中，写明“无”。

保留命中：

- `common/http/response/response_test.go`: `Fail(ctx, nil)` 和 `Fail(ctx, errors.New("sql args password token"))` 是被测生产 helper `response.Fail` 的函数调用，不是 `testify`/`testing.T` 失败断言，符合本变更扫描正则的误报例外。
- `common/http/middleware/auth_test.go`: `fmt.Errorf("validate token version: %w", ...)` 是 mock validator 返回的业务错误构造，不是 `t.Errorf` 或断言失败方法，符合本变更扫描正则的误报例外。
