## Context

`docs/TESTING.md` 和 `openspec/specs/shared-platform-primitives/spec.md` 已要求测试断言与失败处理优先使用 `testify/require`。本次 change 将该规范落到 `common/contract`、`common/validation` 和 `common/testing` 的历史测试文件中，范围横跨共享契约、校验能力和共享测试基础设施，但不改变任何生产包行为。

受影响路径限定为：

- `common/contract/**/*_test.go`
- `common/validation/**/*_test.go`
- `common/testing/**/*_test.go`
- 如需要，`common/go.mod` 和 `common/go.sum`

本次不涉及 `user-service`、`common/http`、`common/runtime`、`common/security`、HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 将目标路径内的常见错误、相等性、集合、fixture 和容器测试断言迁移为 `require.NoError`、`require.ErrorIs`、`require.Equal`、`require.Len`、`require.Contains`、`require.NotNil` 等语义化断言。
- 保留确属测试控制流、特殊诊断输出或测试框架边界的旧失败调用，并在 tasks 验收记录中列明剩余命中及原因。
- 确保 `common` 模块直接声明实际使用的 `testify` 测试依赖，并通过 `go mod tidy` 排除无关依赖漂移。
- 通过目标包测试和 OpenSpec 校验确认迁移完成。

**Non-Goals:**

- 不修改 `common/contract`、`common/validation` 或 `common/testing` 的生产行为、公开 API 或错误语义。
- 不迁移 `common/http`、`common/runtime`、`common/security` 或 `user-service` 测试。
- 不新增兼容旧断言风格的 helper、双写断言或测试专用生产代码。
- 不将手写失败判断机械替换为 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`。

## Decisions

1. 使用 `testify/require` 作为目标范围内阻塞式断言的默认表达。

   理由：`require` 已是项目测试规范，能减少后续空指针和错误状态级联，并让失败信息更稳定。备选方案是保留 `testing` 包手写判断，但会继续产生风格漂移和重复样板。

2. 以语义化断言替换为准，不使用 `Fail` 系列作为通用迁移出口。

   理由：`Fail` 系列只改变失败入口名称，无法表达错误类型、长度、相等性或包含关系等测试意图，也违反 `docs/TESTING.md` 的迁移约束。备选方案是用 `require.Failf` 快速替换所有手写判断，但会降低可读性并掩盖可用的语义断言。

3. 对剩余 `t.Fatal`、`t.Error` 或 `Fail` 命中采用白名单式记录，而不是强行清零。

   理由：测试控制流、特殊诊断输出或测试框架边界可能不适合依赖 `testify` 或无法用现有语义断言清晰表达。备选方案是要求搜索结果完全为零，但会促使不必要的机械替换。

4. 依赖声明只在 `common` 模块内处理。

   理由：目标测试均位于 `common` Go module，若测试实际 import `github.com/stretchr/testify/require`，直接依赖应由 `common/go.mod` 承担。备选方案是在 workspace 根或其他模块处理依赖，但根目录不是单一 Go module，且会扩大变更范围。

## Risks / Trade-offs

- [Risk] 迁移时误把测试控制流改成阻塞断言，改变同一测试中后续检查执行路径。→ Mitigation：逐文件审查旧调用上下文，只将常见断言改为 `require`，需要继续收集多个独立失败的场景保留或使用合适的非阻塞断言。
- [Risk] 容器测试或 fixture helper 的失败边界不适合直接引入 `testify`。→ Mitigation：仅在普通测试断言中引入 `require`，测试基础设施边界的剩余旧调用按例外规则记录。
- [Risk] `go mod tidy` 产生无关依赖漂移。→ Mitigation：检查 `common/go.mod` 和 `common/go.sum` diff，只保留 testify 直接依赖及必要校正。
- [Risk] 搜索命令仍有命中导致验收不清晰。→ Mitigation：在 tasks 中增加剩余命中记录任务，要求逐条确认符合 `docs/TESTING.md` 特殊例外规则。

## Migration Plan

1. 扫描目标路径旧失败调用和现有 `require` 使用情况，建立迁移清单。
2. 在目标测试文件中按语义迁移断言，优先使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.Len`、`require.Contains`、`require.NotNil`、`require.True` 和 `require.False`。
3. 如目标测试新增 `require` import 且 `common/go.mod` 未直接声明 `github.com/stretchr/testify`，补充依赖并在 `common` 模块内运行 `go mod tidy`。
4. 运行搜索验收命令，记录剩余旧失败调用的文件、用途和保留原因。
5. 运行 `go test ./common/contract/... ./common/validation/... ./common/testing/...` 与 `openspec validate standardize-common-contract-test-assertions-no-compat`。

回滚方式：本次只修改测试和可能的测试依赖声明；如迁移导致测试语义异常，可按文件回退对应测试断言修改和依赖变化，不需要数据库、部署或运行时回滚。

## Open Questions

- 无。
