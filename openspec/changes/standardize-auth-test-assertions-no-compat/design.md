## Context

auth 测试覆盖认证会话的核心安全路径，包括登录、refresh token、强制改密、token version、Redis refresh session、PostgreSQL credential store、HTTP response envelope 和 Fx provider 装配。当前这些测试仍大量使用 `t.Fatal` / `t.Errorf` 与手写 if 判断，失败信息和失败后控制流在包之间不一致，也没有让 `docs/TESTING.md` 中的 `testify/require` 断言规范在 auth 范围内落地。

本次变更只迁移测试断言和必要测试依赖声明，不改变生产代码、HTTP API、数据库 schema、Redis key、JWT claims、OpenAPI、部署资产或观测运行时语义。

## Goals / Non-Goals

**Goals:**

- 将 auth 目标范围内的常见错误、对象、布尔、集合、字符串和 HTTP response 检查改为 `require` 或必要时 `assert` 语义化断言。
- 让初始化失败、前置条件失败以及后续检查依赖当前结果的断言使用 `require` 阻塞式失败。
- 仅在自定义测试控制流、特殊诊断输出或不适合引入 `testify` 的测试辅助工具中保留 `t.Fatal` / `t.Error`，并通过扫描命令列明剩余例外。
- 在 `user-service` 模块显式声明 `github.com/stretchr/testify` 测试依赖，并通过 `go mod tidy` 避免无关依赖漂移。

**Non-Goals:**

- 不迁移 user、RBAC、router、cmd、e2e 或 `common` 测试。
- 不修改认证、token version、refresh session、password KDF、Redis 或 PostgreSQL 生产行为。
- 不新增旧 auth HTTP 字段、旧错误码、旧 token 类型、旧状态兼容断言或旧手写断言兼容 helper。
- 不用 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf` 机械替代旧手写失败分支。

## Decisions

1. 使用 `require` 作为默认断言入口，`assert` 仅用于单次测试中需要收集多个相互独立失败的场景。
   - 理由：auth 测试中的多数检查存在前置依赖，例如 err 必须为 nil 后才能检查 token、session、response data 或 DB/Redis 状态；`require` 可以阻断级联失败。
   - 备选方案：全部使用 `assert`。该方案会保留后续空值或错误状态级联失败，不符合 `docs/TESTING.md` 的优先级。

2. 保留少量清晰例外，而不是追求零 `t.Fatal` / `t.Error`。
   - 理由：部分测试辅助工具或自定义诊断输出可能需要直接调用 `testing.TB`，完全消除会降低诊断质量或引入额外包装层。
   - 备选方案：用 `require.FailNowf` 机械替代所有 `t.Fatalf`。该方案违反现有测试规范，也不会提升断言语义。

3. 只在 `user-service` 模块声明 `testify` 依赖，不把依赖或 helper 下沉到 `common`。
   - 理由：本次改动范围是 user-service auth 测试；`common` 已有自身测试依赖，不需要新增跨服务测试 helper 或兼容层。
   - 备选方案：新增共享断言 helper。该方案会制造额外抽象并隐藏标准 `require` 方法，不利于一致失败信息。

4. 迁移以包为单位分批进行，并用扫描命令驱动收敛。
   - 理由：auth 测试横跨 application、transport、infrastructure 和 provider；分批可以保持 diff 可审查，并在每批后运行相关包测试。
   - 备选方案：一次性全仓库替换。该方案容易误改非目标范围，也无法区分允许保留的例外。

## Risks / Trade-offs

- [Risk] 机械迁移可能改变测试控制流，使后续检查不再执行或失败信息不完整。→ Mitigation: 优先选择与原意匹配的 `require.NoError`、`require.ErrorIs`、`require.Equal`、`require.Contains` 等语义化方法；需要多断言收集时才使用 `assert`。
- [Risk] 新增直接依赖可能引起 `go.mod` / `go.sum` 无关漂移。→ Mitigation: 在 `user-service` 目录运行 `go mod tidy`，并检查依赖 diff 只与 `testify` 直接声明相关。
- [Risk] Redis/PostgreSQL adapter 测试依赖真实或半真实测试资源，迁移后可能暴露已有不稳定测试。→ Mitigation: 先保持测试数据、helper 和生产行为不变，只替换断言表达；运行目标 Go 测试确认行为未变。
- [Risk] 剩余 `t.Fatal` / `t.Error` 被误认为未完成。→ Mitigation: 使用验收扫描命令列出剩余命中，并在 tasks 中记录它们符合 `docs/TESTING.md` 的例外原因。

## Migration Plan

1. 创建 OpenSpec change artifacts 并通过 `openspec validate standardize-auth-test-assertions-no-compat`。
2. 在 `user-service` 模块声明 `github.com/stretchr/testify` 直接测试依赖并运行 `go mod tidy`。
3. 分批迁移 auth application/domain、transport/http、infrastructure/postgres、infrastructure/redis、metrics 和 provider 测试断言。
4. 执行断言扫描，消除不必要的 `t.Fatal` / `t.Error` / `Fail` 命中，并记录允许保留的例外。
5. 运行 `go test ./user-service/internal/features/auth/... ./user-service/internal/providers` 和必要的架构/OpenSpec 验证。

回滚方式是还原本 change 修改的测试断言、`user-service/go.mod` / `go.sum` 和 OpenSpec change 目录；由于不涉及生产行为或数据结构，无需发布迁移或运行时回滚步骤。

## Open Questions

- 无。
