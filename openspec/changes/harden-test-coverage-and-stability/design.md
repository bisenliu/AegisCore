## Context

本 change 源于三类已确认问题。第一，`tools/openapi-convert` 属于仓库级交付工具，当前无 `*_test.go`，`go test -cover ./...` 覆盖率为 0%，且根 `Makefile test/verify` 未执行该模块测试。第二，role infrastructure 默认覆盖率为 55.6%，但 `RolePermissionStore` 的 `Add`、`ListByRoleID`、`Replace`、`Remove` 在默认环境下覆盖率为 0%，原因是 PostgreSQL 容器测试需要 `AEGISCORE_TEST_CONTAINERS=1` 才执行。第三，部分测试仍使用固定 `time.Sleep` 或手动 `os.Setenv` 恢复全局环境，虽然重复运行未稳定复现失败，但这些写法会在高负载、调度延迟或未来并行测试中引入非确定性。

本 change 横跨 `tools/openapi-convert`、根 `Makefile`、`user-service/internal/features/role/infrastructure/postgres`、`common/runtime/*` 和 auth 测试。它不改变 HTTP API、数据库 schema、OpenAPI 输出语义、Redis key schema、Casbin policy 或部署资产运行时行为。

## Goals / Non-Goals

**Goals:**

- `tools/openapi-convert` 具备参数解析、错误路径和文件生成回归测试，并被根 `make test` 与 `make verify` 覆盖。
- role infrastructure 默认测试覆盖 `RolePermissionStore` 中不依赖 PostgreSQL 行锁的查询、删除、系统绑定同步、缺失权限、失败保持和映射 helper 路径，使默认覆盖率达到 70%+，且不为了 SQLite 测试改变生产锁语义。
- `time.Sleep` 和手动 `os.Setenv` 命中的测试改为基于可观察条件、通道、确定性时间/score 或 `t.Setenv` 的稳定测试。
- 不保留旧测试形态的兼容 helper 或绕过验证链的工具模块行为。

**Non-Goals:**

- 不新增 HTTP API，不调整 OpenAPI 外部契约，不修改 Ent schema 或 Atlas migration。
- 不引入新的外部依赖、eventbus、outbox、worker 或部署资产。
- 不为了覆盖率在生产代码中加入 `NewXForTest`、`testHook`、全局可变时钟或仅测试使用的业务无关分支。
- 不要求普通默认测试启动 Docker；需要真实 PostgreSQL 锁语义的集成测试可继续通过显式环境变量运行。

## Decisions

### Decision: `tools/openapi-convert` 使用可测试的 run 函数

`main()` 只负责调用 `os.Exit(run(...))`，参数解析、转换执行、stdout/stderr 输出和错误码返回放入可测试函数。该函数使用 `flag.NewFlagSet`，避免全局 `flag` 状态污染测试。

选择该方案是因为它不改变 CLI 外部行为，却能直接覆盖参数解析和错误路径。备选方案是通过 `os/exec` 构建临时二进制做黑盒测试，但会增加测试耗时、平台差异和错误定位成本。

### Decision: 根验证链显式纳入 tools 模块

根 `Makefile` 增加 `tools-openapi-convert-test`，`test` 依赖 `common-test`、`user-service-test` 和 `tools-openapi-convert-test`。`verify` 继续依赖 `test`，从而自动覆盖 tools 模块。

选择该方案是因为 `go.work` 已包含 `./tools/openapi-convert`，但仓库根不是单一 Go module，`go test ./...` 在根目录不可用。显式 Make target 能与现有 common/user-service 模块验证风格一致。

### Decision: role 绑定关键路径优先使用默认可执行测试覆盖

`RolePermissionStore` 的默认测试只覆盖不依赖 PostgreSQL `FOR UPDATE` 语义的行为，例如查询、删除、系统绑定同步、缺失权限和映射 helper。`Add` 与 `Replace` 的真实行锁路径保持生产实现不变，由现有 Docker-backed PostgreSQL 测试覆盖。

选择该方案是因为默认 CI 和本地 `go test` 不应因未启用 Docker 而失去基础持久化路径覆盖，也不应为了 SQLite 测试在生产代码中加入方言降级分支。备选方案是让生产查询在 SQLite 下跳过 `FOR UPDATE`，但该方案会把测试方言兼容带入生产路径，因此不采用。

### Decision: 异步和时间相关测试使用条件等待或确定性输入

固定 `time.Sleep` 替换为 `require.Eventually`、条件通道、`time.After` 超时保护或确定性 Redis score/可注入输入。缓存过期场景如果无法完全消除真实 TTL，应通过 eventually-style 条件断言降低调度假设，而不是硬编码 sleep 后立即断言。

选择该方案是因为它能表达“等待条件达成”而不是“等待固定时间”。备选方案是简单增加 sleep 时长，但这会拉长测试且仍不能消除调度非确定性。

### Decision: 全局环境测试使用 `t.Setenv` 并显式恢复非环境全局状态

`TZ` 环境变量由 `t.Setenv` 管理，`time.Local` 和包级 `state` 仍通过 `t.Cleanup` 恢复。相关测试不得使用 `t.Parallel`。

选择该方案是因为 `t.Setenv` 能由 testing 框架负责环境恢复，减少手动 `os.Setenv`/`os.Unsetenv` 分支遗漏；`time.Local` 和包级状态不是环境变量，仍需显式隔离。备选方案是保留手动恢复，但该方案对未来并行化和失败中断更脆弱。

## Risks / Trade-offs

- 默认 role 绑定测试使用 SQLite/Ent 测试数据库无法覆盖 PostgreSQL `FOR UPDATE` 锁语义 → 保留显式 Docker-backed PostgreSQL 集成测试用于真实数据库语义，默认测试只覆盖非锁定持久化路径和错误映射。
- `require.Eventually` 仍依赖超时时间 → 使用较短 poll interval 和明确失败诊断，优先在可行处改为通道信号或确定性输入。
- `tools/openapi-convert` run 函数拆分可能触碰 CLI 退出码和 stderr 文案 → 测试固定必填参数、约束错误和成功输出，保持既有命令行行为。
- 根 `make test` 增加 tools 模块会增加少量执行时间 → 工具模块测试无外部依赖，成本应低于长期回归风险。

## Migration Plan

1. 调整 `tools/openapi-convert` 为可测试结构并新增测试。
2. 将 tools 模块测试接入根 `Makefile`。
3. 补齐 role infrastructure 默认可执行测试和必要的事务失败断言。
4. 替换固定 `time.Sleep` 和手动 `os.Setenv` 测试模式。
5. 运行相关包测试和覆盖率命令，最后暂存预期变更后执行 `make lint` 与 `make verify`。

回滚方式是回退本 change 的代码、测试、Makefile 和 OpenSpec artifacts；由于不涉及 schema、API 或部署状态，回滚不需要数据迁移或运行时兼容处理。

## Open Questions

无。
