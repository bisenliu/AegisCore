## Context

`user-service/cmd/main.go` 当前在 package scope 定义 `newLifecycleApp`、`runRBACSeed`、`runAssignSuperAdmin` 和 `runCreateSuperAdmin`，生产代码通过这些变量构造 `serve` 与 `rbac` 子命令，测试则保存原值、全局赋值替换、再用 `t.Cleanup` 恢复。这使 `user-service/cmd` 的根命令测试共享可变状态，无法具备并行化前提，也会让 race 测试暴露不必要的全局写入风险。

本次变更只影响 `user-service/cmd` 的 CLI 装配和测试替身注入方式。RBAC seed、超级管理员创建、数据库连接、Ent、Casbin、HTTP API、OpenAPI、部署清单和 migration 均不改变。

## Goals / Non-Goals

**Goals:**

- 用显式命令依赖结构替代 root command 测试对 package-level 可变函数变量的赋值。
- 让 `serve` 的 lifecycle app factory 和 RBAC 子命令 runner 在每个 root command 实例内局部绑定。
- 保持 `serve`、`rbac seed`、`rbac assign-super-admin`、`rbac create-super-admin` 和 `fxgraph` 的现有 CLI 行为。
- 更新 `user-service/cmd/main_test.go`，使目标测试不再通过保存/恢复全局函数替身表达依赖。

**Non-Goals:**

- 不重构 `runRBACSeedCommand`、`runAssignSuperAdminCommand`、`runCreateSuperAdminCommand` 的数据库装配和业务流程。
- 不移除或调整 `rbac.go` 中既有 RBAC 依赖装配测试 hook。
- 不改变 Cobra command 名称、flag、默认值、输出语义或退出码。
- 不修改 Ent schema、Atlas migration、OpenAPI 生成物、部署资产或 feature 业务代码。

## Decisions

1. 在 `main.go` 引入未导出的 `rootCommandDependencies`，字段包括 lifecycle app factory 与三个 RBAC runner。
   - 理由：依赖结构是 root command 构造的运行时职责，生产入口可使用默认依赖，测试可按用例传入局部替身。
   - 备选方案：保留全局变量并加锁。该方案仍让测试共享全局状态，不能满足并行化前提。

2. `newRootCommand` 接收依赖结构并在函数开头填充缺省值。
   - 理由：调用方可以只覆写需要的字段，生产 `main()` 仍能通过默认构造获得完整命令图。
   - 备选方案：新增 `newTestRootCommand`。该方案会产生测试专用生产 API，不符合架构边界中避免测试驱动冗余生产代码的约束。

3. `runServe` 接收 lifecycle app factory 参数，由 `serve` command 的 RunE 从依赖结构传入。
   - 理由：`runServe` 本身只需要最小 lifecycle factory，不需要知道 root command 或 Cobra 细节。
   - 备选方案：把 `runServe` 包成结构体方法。当前只有单一函数调用点，结构体方法会引入不必要的抽象。

4. RBAC 子命令继续调用现有真实 runner 函数，但 runner 函数值从 command dependency 捕获。
   - 理由：保留 `rbac.go` 的业务边界和 cleanup/error 语义，只改变 command wiring 的依赖注入方式。
   - 备选方案：把 RBAC runner 进一步拆分为接口和 adapter。本次不需要扩展业务协作者，额外接口会超过目标范围。

## Risks / Trade-offs

- [Risk] 依赖结构字段遗漏默认值会导致生产命令空指针或未执行真实 runner。→ 在 `defaultRootCommandDependencies` 中集中声明默认依赖，并由 `newRootCommand` 统一归一化。
- [Risk] 改动 root command 签名会影响现有测试和 `main()` 调用。→ 同步更新调用点，并通过 `go test -race ./cmd` 覆盖命令构造、flag 传递和 serve 生命周期。
- [Risk] spec delta 可能把一次性重构写成过细业务契约。→ delta 只描述长期测试隔离和 CLI 行为保持约束，不固化具体字段名之外的业务实现细节。

## Migration Plan

本次变更是源码级重构，不需要数据迁移或部署步骤。回滚时可恢复旧的 root command 构造和测试替身方式；由于 CLI 外部行为不变，运行时回滚无需数据库、OpenAPI 或部署资产配合。

验证方式：

- `go test -race ./cmd`，在 `user-service` 模块下运行。
- `make user-service-architecture-lint`，在仓库根目录运行。

## Open Questions

- 无。
