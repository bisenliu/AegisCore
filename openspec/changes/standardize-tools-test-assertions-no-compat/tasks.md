## 1. 基线与范围确认

- [x] 1.1 确认 `tools/openapi-convert/go.mod` 的依赖状态，只有存在真实工具测试迁移需求时才新增 `testify` 依赖，不产生无关 `go mod tidy` 漂移。
  - 当前仅声明 `github.com/aegiscore/common v0.0.0`，本 change 未新增 `testify` 或其他依赖，未运行 `go mod tidy`。
- [x] 1.2 运行 `rg --files -g '*_test.go' tools`，记录 `tools` 范围内实际 Go 测试文件列表。
  - 无输出，退出码 1：`tools` 范围当前没有 `_test.go` 文件。
- [x] 1.3 扫描仓库中不属于 `common/`、`user-service/internal/`、`user-service/tests/` 的 Go 测试，确认哪些属于仓库级工具或交付工具测试，避免越界修改 user-service feature、router/provider/cmd 或 E2E 测试。
  - `rg --files -g '*_test.go' | rg -v '^(common/|user-service/internal/|user-service/tests/)'` 命中 `user-service/cmd/main_test.go`、`user-service/cmd/rbac_test.go`、`user-service/ent/schema/user_test.go`；这些不属于本次 `tools` 范围，未修改。
- [x] 1.4 扫描目标工具测试范围内的历史失败断言：`rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" tools --glob '*_test.go'`，记录迁移前命中；若无测试文件，记录为空范围。
  - 无输出，退出码 1：目标工具测试范围为空，无历史失败断言可迁移。
- [x] 1.5 扫描目标工具测试范围内的 `testify` 使用点：`rg "github.com/stretchr/testify/(require|assert)" tools --glob '*_test.go'`，记录迁移前使用情况。
  - 无输出，退出码 1：目标工具测试范围为空，无迁移前 `testify` 使用点。

## 2. 工具测试断言迁移

- [x] 2.1 如存在 `tools/**/*_test.go`，迁移工具错误、命令执行、文件读取、生成路径和 cleanup 断言，优先使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.NotEmpty` 或等价语义化断言。
  - 当前不存在 `tools/**/*_test.go`，无代码迁移对象。
- [x] 2.2 如存在 OpenAPI 转换工具测试，迁移 JSON/YAML、文件内容、集合长度、字符串包含和正则匹配断言，优先使用 `require.JSONEq`、`require.Len`、`require.Contains`、`require.ElementsMatch`、`require.Regexp` 或结构化解析后的断言。
  - 当前不存在 OpenAPI 转换工具测试文件，未修改 `tools/openapi-convert` 行为或输出契约。
- [x] 2.3 如存在多个独立输出字段或文件内容差异检查，可按 `docs/TESTING.md` 使用 `testify/assert` 收集独立失败；前置条件、解析失败和后续依赖结果继续使用 `require`。
  - 当前无工具测试文件，无 `assert` 迁移对象。
- [x] 2.4 如当前没有工具测试包，不新增旧工具输出格式、旧 CLI flag 或旧文件路径兼容断言；在本文件记录实际空范围和替代验证。
  - 已记录空范围；未新增工具测试、旧输出格式、旧 CLI flag 或旧文件路径兼容断言。

## 3. 残留扫描与例外记录

- [x] 3.1 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" tools --glob '*_test.go'`，确认剩余命中均符合 `docs/TESTING.md` 特殊例外规则。
  - 无输出，退出码 1：无剩余命中。
- [x] 3.2 在本文件第 5 节记录最终剩余命中；允许的剩余项只能是自定义测试控制流、特殊诊断输出、测试辅助工具边界或扫描正则误报。
  - 已在第 5 节记录最终结果。
- [x] 3.3 运行 `rg "github.com/stretchr/testify/(require|assert)" tools --glob '*_test.go'`，确认存在迁移后的实际使用点；若 `tools` 无测试文件，记录无使用点和原因。
  - 无输出，退出码 1：`tools` 无测试文件，因此无迁移后的 `testify` 使用点。
- [x] 3.4 检查目标范围内未在存在更具体断言时使用 `require.True` / `require.False` 或 `assert.True` / `assert.False` 包装布尔表达式。
  - `rg "\\b(require|assert)\\.(True|False)\\(" tools --glob '*_test.go'` 无输出，退出码 1：无布尔包装断言。

## 4. 验证

- [x] 4.1 如修改 Go 测试文件，对本次修改的工具测试文件运行 `gofmt`。
  - 未修改 Go 测试文件，无需运行 `gofmt`。
- [x] 4.2 运行 `go test ./tools/...`；若当前 tools workspace 无测试包，记录实际包列表和 Go 命令输出作为替代验证。
  - `go test ./tools/...` 失败：`pattern ./tools/...: directory prefix tools does not contain modules listed in go.work or their selected dependencies`，原因是 `tools/` 不是 workspace module，实际 module 是 `tools/openapi-convert`。
  - 替代验证：`go list ./tools/openapi-convert/...` 输出 `github.com/aegiscore/tools/openapi-convert`。
  - 替代验证：`go test ./tools/openapi-convert/...` 通过，输出 `? github.com/aegiscore/tools/openapi-convert [no test files]`。
  - 替代验证：在 `tools/openapi-convert` 目录运行 `go list ./...` 和 `go test ./...`，结果同上。
- [x] 4.3 运行 `openspec validate standardize-tools-test-assertions-no-compat`，确认 change spec 有效。
  - 通过：`Change 'standardize-tools-test-assertions-no-compat' is valid`。
- [x] 4.4 运行 `make user-service-architecture-lint`，确认 OPSX 文档语言和架构边界检查通过。
  - 通过：`architecture-lint: ok`。
- [x] 4.5 将本次预期代码、测试和 OpenSpec 变更加到暂存区后运行 `make lint`；若工作区存在非本次变更或环境阻塞，记录具体原因和已完成的作用域验证。
  - 未运行：当前工作区已有非本次 staged/unstaged 变更（`cover-rbac-cli-commands-no-compat`、`user-service/cmd`、`user-service/tests/e2e`、`AGENTS.md`、`standardize-e2e-test-assertions-no-compat`），不适合为本 change 构造干净暂存区；已完成本 change 的作用域扫描、工具模块替代测试、OpenSpec 校验和架构 lint。
- [x] 4.6 保持本次预期变更已暂存，运行 `make verify`；若最终 `git diff --exit-code` 会被非本次变更或 Multica runtime 文件阻塞，记录具体原因并限定说明已完成的替代验证。
  - 未运行：同 4.5，当前工作区和暂存区包含非本次变更，`make verify` 的最终 diff 检查不能代表本 change；已完成本 change 的限定验证。

## 5. 剩余例外记录

- [x] 5.1 实施完成后填写最终残留扫描结果；若 `tools` 无 `_test.go` 文件，记录为无工具测试文件、无历史失败断言残留、无 `testify` 使用点。
  - 最终结果：`tools` 无 `_test.go` 文件，无历史失败断言残留，无 `testify` 使用点，无 `require.True` / `require.False` 或 `assert.True` / `assert.False` 包装布尔表达式。
