## Context

`docs/TESTING.md` 和 `delivery-operations` 主规格已经要求 Go 测试优先使用 `testify/require` 的语义化断言，避免手写失败判断和可被专属断言替代的 `True` / `False`。本 change 将该规范落到仓库级工具测试范围，重点覆盖 `tools/openapi-convert`、OpenAPI 转换输入输出、JSON/YAML 内容、生成文件路径和交付脚本相关 Go 测试。

当前工作区中 `tools/` 只有 `tools/openapi-convert/go.mod` 与 `main.go`，没有 `tools/**/*_test.go`。仓库内不属于 `common/`、`user-service/internal/`、`user-service/tests/` 的 Go 测试当前主要是 `user-service/cmd/*_test.go` 与 Ent schema 测试，已由其他 change 或既有主规格边界覆盖；本 change 不迁移这些不属于仓库级 tools 的测试。

## Goals / Non-Goals

**Goals:**

- 扫描并迁移仓库级工具测试中的历史手写断言与模糊布尔断言。
- 对工具错误、文件内容、JSON/YAML 输出、集合长度、字符串匹配和生成路径校验使用更具体的 `require` / `assert` 断言。
- 在当前无工具测试包时，记录真实扫描结果和替代验证，确保 change 可审计。
- 保持工具输出、CLI flag、OpenAPI 生成物和部署资产不变。

**Non-Goals:**

- 不修改 `tools/openapi-convert` 的生产行为或兼容旧输出格式。
- 不迁移 `common/`、`user-service/internal/`、`user-service/tests/` 中的测试。
- 不把手写失败判断机械替换为 `Fail` / `Failf` / `FailNow` / `FailNowf`。
- 不新增仅为测试服务的生产代码、接口、分支或 helper。

## Decisions

1. 工具测试迁移只处理真实存在的 `_test.go` 文件。
   - 原因：当前 `tools/` 无 Go 测试包，新增虚假测试或兼容旧输出断言会扩大行为面。
   - 备选方案：为 `tools/openapi-convert` 补充新测试。该方案会新增覆盖范围而非迁移历史断言，超出本 issue 的“不修改工具生产行为/旧兼容断言”边界。

2. 扫描命令按路径限定工具范围。
   - 原因：`rg "t\\.Fatalf|..." tools --glob '*_test.go'` 是验收条件；当前没有文件时应记录空结果，而不是扫描到 user-service CLI 或 common 测试后越界修改。
   - 备选方案：扫描全仓库所有 `_test.go`。该方案会触及其他已独立拆分的测试断言迁移 change，容易与现有工作区变更冲突。

3. 后续新增工具测试时优先使用语义化 `require`，必要时使用 `assert` 收集独立输出差异。
   - 原因：工具测试常见失败点是错误消息、文件内容、JSON/YAML 等结构化输出，专属断言能提供更稳定的失败信息。
   - 备选方案：允许 `require.True` 包装 `strings.Contains`、`len(...)` 或 JSON 字符串比较。该方案失败信息弱，且违反既有测试规范。

## Risks / Trade-offs

- [Risk] 当前无可迁移工具测试，change 的代码 diff 可能只有 OpenSpec 产物与 tasks 状态。→ Mitigation：在 tasks 中记录 `rg --files -g '*_test.go' tools`、断言扫描和 `go test ./tools/...` 的实际结果。
- [Risk] 工作区已有其他 change 修改 `user-service/cmd`、E2E 或 OpenSpec 目录。→ Mitigation：只修改 `openspec/changes/standardize-tools-test-assertions-no-compat/` 与必要的工具测试文件，不清理或重写无关变更。
- [Risk] `go test ./tools/...` 在 workspace 工具模块布局下没有测试包但仍需验证工具模块可测试。→ Mitigation：同时记录 `go list ./tools/...` 或 Go 命令输出，作为无测试包场景的替代验证依据。

## Migration Plan

1. 创建 OpenSpec change artifacts。
2. 定位 `tools` 与其他仓库级工具测试文件。
3. 如存在历史断言，按 `docs/TESTING.md` 迁移为语义化 `require` / `assert`。
4. 如不存在工具测试包，记录空范围与替代验证。
5. 运行断言扫描、`go test ./tools/...`、`openspec validate standardize-tools-test-assertions-no-compat`。

回滚方式：删除本 change 目录及本 change 引入的工具测试断言迁移；因不修改生产行为、schema、部署和 OpenAPI 生成物，不需要运行时回滚。

## Open Questions

- 无。
