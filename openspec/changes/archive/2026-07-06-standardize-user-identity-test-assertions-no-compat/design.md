## Context

user 与 shared identity 测试覆盖用户资料创建、查询、列表、状态约束、软删除过滤、HTTP response envelope、PostgreSQL adapter 和身份状态判断。当前这些测试仍存在 `t.Fatal` / `t.Errorf` 与手写 if 判断，失败信息和失败后控制流在 domain、transport/http、infrastructure/postgres 与 `internal/shared/identity` 之间不一致，也没有让 `docs/TESTING.md` 中的 `testify/require` 断言规范在 user 身份边界内充分落地。

本次变更只迁移测试断言和必要测试依赖声明，不改变生产代码、HTTP API、OpenAPI、数据库 schema、Atlas migration、用户状态、软删除、分页、response envelope、部署资产或观测运行时语义。

受影响路径限定为：

- `user-service/internal/features/user/**/*_test.go`
- `user-service/internal/shared/identity/**/*_test.go`
- 如需要，`user-service/go.mod` 和 `user-service/go.sum`

## Goals / Non-Goals

**Goals:**

- 将目标范围内的常见错误、对象、布尔、集合、字符串、HTTP response 和 pagination 检查改为 `require` 或必要时 `assert` 语义化断言。
- 让初始化失败、前置条件失败以及后续检查依赖当前结果的断言使用 `require` 阻塞式失败。
- 对同一个 response 或 DTO 中多个互不依赖字段需要同时收集失败的场景，按 `docs/TESTING.md` 使用 `assert`。
- 仅在自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的场景中保留 `t.Fatal` / `t.Error` / `Fail`，并通过扫描命令列明剩余例外。
- 在 `user-service` 模块显式声明 `github.com/stretchr/testify` 测试依赖，并通过 `go mod tidy` 避免无关依赖漂移。

**Non-Goals:**

- 不迁移 auth、RBAC、router、cmd、e2e、`common` 或 `user-service/internal/providers` 测试。
- 不修改用户资料、identity 状态、软删除、分页、HTTP envelope、数据库 schema、Ent predicate、OpenAPI 或 HTTP API 行为。
- 不新增旧 user 字段、旧状态兼容断言、旧响应 envelope 断言或旧手写断言兼容 helper。
- 不用 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf` 机械替代旧手写失败分支。

## Decisions

1. 使用 `require` 作为默认断言入口，`assert` 仅用于单次测试中需要收集多个相互独立失败的 response/DTO 字段。

   理由：目标测试中的多数检查存在前置依赖，例如 err 必须为 nil 后才能检查用户对象、response data、pagination 或 DB 状态；`require` 可以阻断级联失败。备选方案是全部使用 `assert`，但会保留后续空值或错误状态级联失败，不符合 `docs/TESTING.md` 的优先级。

2. 按包边界分批迁移 domain/shared identity、transport/http 和 infrastructure/postgres 测试。

   理由：这些测试分别关注状态模型、HTTP 边界和持久化语义；分批迁移可以让 diff 与验证命令更可审查，并降低误改生产语义的风险。备选方案是一次性全仓库替换，但会扩大范围并难以区分允许保留的例外。

3. 保留少量清晰例外，而不是追求零 `t.Fatal` / `t.Error` / `Fail`。

   理由：部分测试辅助工具或自定义诊断输出可能需要直接调用 `testing.TB`，完全消除会降低诊断质量或引入额外包装层。备选方案是用 `require.FailNowf` 机械替代所有旧失败调用，该方案不表达断言语义，也违反现有测试规范。

4. 只在 `user-service` 模块声明 `testify` 依赖，不把依赖或 helper 下沉到 `common`。

   理由：目标测试均位于 `user-service` Go module，直接依赖应由该 module 承担；新增共享断言 helper 会隐藏标准断言语义并扩大边界。备选方案是新增 `common/testing` 断言包装器，但会制造兼容层并与 no-compat 目标冲突。

## Risks / Trade-offs

- [Risk] 机械迁移可能改变测试控制流，使后续检查不再执行或失败信息不完整。→ Mitigation: 优先选择与原意匹配的 `require.NoError`、`require.ErrorIs`、`require.Equal`、`require.Contains`、`require.Len` 等语义化方法；需要多断言收集时才使用 `assert`。
- [Risk] HTTP response 或 pagination 测试中前置解码失败后继续访问字段会产生级联失败。→ Mitigation: 解码、类型断言和 data/pagination 存在性使用 `require`，互不依赖字段值检查才使用 `assert`。
- [Risk] PostgreSQL adapter 测试迁移后可能暴露既有测试资源或数据顺序不稳定问题。→ Mitigation: 保持测试数据、helper、Ent 查询和生产行为不变，只替换断言表达；运行目标 Go 测试确认行为未变。
- [Risk] `go mod tidy` 产生无关依赖漂移。→ Mitigation: 检查 `user-service/go.mod` / `user-service/go.sum` diff，只保留 testify 直接依赖及必要校正。
- [Risk] 剩余 `t.Fatal` / `t.Error` / `Fail` 被误认为未完成。→ Mitigation: 使用验收扫描命令列出剩余命中，并在 tasks 中记录它们符合 `docs/TESTING.md` 的例外原因。

## Migration Plan

1. 创建 OpenSpec change artifacts 并通过 `openspec validate standardize-user-identity-test-assertions-no-compat`。
2. 扫描目标路径旧失败调用和现有 `require` / `assert` 使用情况，建立迁移清单。
3. 如目标测试新增 `require` / `assert` import 且 `user-service/go.mod` 未直接声明 `github.com/stretchr/testify`，补充依赖并在 `user-service` 模块内运行 `go mod tidy`。
4. 分批迁移 user domain、HTTP transport、PostgreSQL infrastructure 和 shared identity 测试断言。
5. 执行断言扫描，消除不必要的 `t.Fatal` / `t.Error` / `Fail` 命中，并记录允许保留的例外。
6. 运行 `go test ./user-service/internal/features/user/... ./user-service/internal/shared/identity/...` 和必要的 OpenSpec/架构验证。

回滚方式是还原本 change 修改的测试断言、`user-service/go.mod` / `go.sum` 和 OpenSpec change 目录；由于不涉及生产行为或数据结构，无需数据库、部署或运行时回滚步骤。

## Open Questions

- 无。
