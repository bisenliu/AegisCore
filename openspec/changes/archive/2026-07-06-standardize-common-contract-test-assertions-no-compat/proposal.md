## Why

`common/contract`、`common/validation` 和 `common/testing` 中仍有历史测试使用手写 `t.Fatal`、`t.Error` 或等价失败判断表达常见断言，和已固化的测试断言规范不一致。现在集中迁移这些共享基础包的测试，可以减少样板代码和诊断差异，避免后续新增或重构时继续扩散旧断言风格。

## What Changes

- 将 `common/contract/**/*_test.go`、`common/validation/**/*_test.go` 和 `common/testing/**/*_test.go` 中的常见错误、相等性、集合、fixture 和容器测试断言迁移为 `testify/require` 语义化方法。
- 如 `common/go.mod` 尚未直接声明 `github.com/stretchr/testify`，补充直接测试依赖，并通过 `go mod tidy` 确认依赖文件没有无关漂移。
- 保留少量确属测试控制流、特殊诊断输出或测试框架边界的 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf` 或 `Fail` 用法，并在任务记录中列明剩余命中及保留理由。
- 不新增旧断言风格兼容 helper，不把手写失败判断机械替换为 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`。
- 不修改 `common/contract`、`common/validation` 或 `common/testing` 的生产行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 收紧共享基础包测试断言与失败处理风格在 `common/contract`、`common/validation` 和 `common/testing` 历史测试中的落地要求，明确这些范围内的常见阻塞式断言必须优先使用 `testify/require`。

## Impact

- 受影响测试路径：`common/contract/**/*_test.go`、`common/validation/**/*_test.go`、`common/testing/**/*_test.go`。
- 受影响依赖：可能需要在 `common/go.mod` 中直接声明 `github.com/stretchr/testify` 测试依赖，并同步 `common/go.sum`。
- 不影响外部 API、HTTP 契约、数据库 schema、OpenAPI、部署资产、观测资产或生产运行时语义。
- 验证包括目标范围旧失败调用搜索、`testify/require` 使用点搜索、目标 Go 测试和 OpenSpec 校验。
