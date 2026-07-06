## Why

仓库级工具测试需要与 `docs/TESTING.md` 中固化的 Go 测试断言规范保持一致，避免继续使用手写 `t.Fatal` / `t.Errorf` 或模糊布尔断言表达 OpenAPI 转换、CLI 输入输出和文件生成校验。当前 `tools/` 模块没有 `_test.go`，本 change 用于固化工具测试断言边界、验证当前缺口，并确保后续新增工具测试沿用统一规范。

## What Changes

- 检查 `tools/**/*_test.go` 以及仓库中不属于 `common/`、`user-service/internal/`、`user-service/tests/` 的 Go 工具测试断言使用情况。
- 需要迁移的工具测试优先使用 `testify/require` 的语义化断言，例如 `Len`、`ErrorContains`、`Contains`、`ElementsMatch`、`JSONEq`、`Regexp` 等。
- 对多个独立输出字段或文件内容差异检查，可按 `docs/TESTING.md` 使用 `testify/assert` 收集独立失败。
- 不修改工具生产行为、OpenAPI 生成物、部署资产、CLI flag 或旧输出兼容逻辑。
- 当前如无工具测试包，则在 tasks 中记录实际包列表、扫描结果和替代验证，不伪造迁移对象。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`：明确仓库级工具测试也必须遵循统一 Go 测试断言规范，并将无测试包场景纳入交付验证记录。
- `runtime-observability`：明确 OpenAPI 转换和生成链路相关工具测试在校验 JSON/YAML、文件内容和生成路径时必须使用语义化断言，且不改变 OpenAPI 文档运行时或生成物契约。

## Impact

- 影响范围：`tools/**/*_test.go`、其他非 `common/` / 非 `user-service/internal/` / 非 `user-service/tests/` 的 Go 工具测试，以及 `openspec/changes/standardize-tools-test-assertions-no-compat/`。
- 不影响 HTTP API、数据库 schema、Ent/Atlas migration、RBAC 授权、OpenAPI 生成物、部署资产或 CLI flag。
- 验证需要覆盖断言扫描、`go test ./tools/...`、`openspec validate standardize-tools-test-assertions-no-compat`，并在工具测试包为空时记录 `go list` / `go test` 的实际结果。
