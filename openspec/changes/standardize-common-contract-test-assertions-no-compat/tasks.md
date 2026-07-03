## 1. 范围扫描

- [x] 1.1 扫描 `common/contract`、`common/validation` 和 `common/testing` 目标测试文件中的旧失败调用：`rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/contract common/validation common/testing --glob '*_test.go'`。
- [x] 1.2 扫描目标测试文件中现有 `testify/require` 使用点：`rg "github.com/stretchr/testify/require" common/contract common/validation common/testing --glob '*_test.go'`。
- [x] 1.3 确认 `common/go.mod` 是否已直接声明 `github.com/stretchr/testify`，并记录是否需要调整依赖。

## 2. 断言迁移

- [x] 2.1 将 `common/contract/**/*_test.go` 中常见错误、相等性、集合和状态断言迁移为语义化 `require` 方法，不修改生产代码。
- [x] 2.2 将 `common/validation/**/*_test.go` 中常见错误、相等性、集合和状态断言迁移为语义化 `require` 方法，不修改生产代码。
- [x] 2.3 将 `common/testing/**/*_test.go` 中 fixture、容器和测试基础设施相关的常见断言迁移为语义化 `require` 方法，不修改生产代码。
- [x] 2.4 对无法通过语义化 `require` 或 `assert` 清晰表达的测试控制流、特殊诊断输出或测试框架边界，保留旧失败调用并确保代码上下文能说明原因。
- [x] 2.5 确认迁移过程未引入 `require.Fail`、`require.Failf`、`assert.Fail`、`assert.Failf` 或旧断言风格兼容 helper。

## 3. 依赖整理

- [x] 3.1 如目标测试新增或已有 `require` import 且 `common/go.mod` 未直接声明 `github.com/stretchr/testify`，在 `common` 模块补充直接测试依赖。
- [x] 3.2 在 `common` 模块运行 `go mod tidy`，检查 `common/go.mod` 和 `common/go.sum` diff，只保留 testify 相关直接依赖及必要校正。

## 4. 验收验证

- [x] 4.1 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/contract common/validation common/testing --glob '*_test.go'`，逐条记录剩余命中及其符合 `docs/TESTING.md` 特殊例外规则的原因。剩余命中：无。
- [x] 4.2 运行 `rg "github.com/stretchr/testify/require" common/contract common/validation common/testing --glob '*_test.go'`，确认能定位到迁移后的实际使用点。
- [x] 4.3 运行 `go test ./common/contract/... ./common/validation/... ./common/testing/...`，确认目标包测试通过。
- [x] 4.4 运行 `openspec validate standardize-common-contract-test-assertions-no-compat`，确认 change 通过 OpenSpec 校验。

## 5. 最终检查

- [x] 5.1 检查 `git diff`，确认变更仅包含目标测试、必要的 `common` 依赖文件和本 change artifacts，不包含生产行为、API、数据库、OpenAPI、部署或观测资产变更。
- [x] 5.2 将本次预期代码和文档变更加到暂存区后运行 `make lint`，确认 lint 通过。
- [x] 5.3 在预期变更已暂存的前提下运行 `make verify`，确认完整验证通过且没有未预期 drift。
