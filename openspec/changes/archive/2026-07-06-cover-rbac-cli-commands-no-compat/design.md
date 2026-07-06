## Context

`user-service/cmd/main.go` 挂载当前 `user-service rbac` 命令，`user-service/cmd/rbac.go` 负责 RBAC seed、超级管理员角色绑定和默认超级管理员创建。现有测试已经覆盖 root command 表面和少量 flag 传递，但 `runRBACSeedCommand`、`runAssignSuperAdminCommand`、`runCreateSuperAdminCommand`、`createSuperAdmin`、`normalizeCreateSuperAdminOptions` 和 `chainCleanup` 的真实 runner、依赖装配失败、参数归一化和 cleanup 合并语义仍缺少直接覆盖。

本变更只补齐命令测试和必要的可测试性 seam，不改变 RBAC seed 业务语义、role/permission baseline、超级管理员默认绑定、HTTP API、OpenAPI、数据库 schema、migration 或部署资产。测试必须使用当前 `user-service rbac` 命令契约，不为旧命令名、旧环境变量、旧 root Makefile 无服务前缀入口或旧 bootstrap 行为保留兼容路径。

## Goals / Non-Goals

**Goals:**

- 覆盖 `runRBACSeedCommand`、`runAssignSuperAdminCommand` 和 `runCreateSuperAdminCommand` 的成功、配置错误、依赖初始化错误和 cleanup 错误合并路径。
- 覆盖 `createSuperAdmin` 的已有用户绑定、新建用户绑定、已有用户重置密码和错误传播路径。
- 覆盖 `normalizeCreateSuperAdminOptions` 的 username、nickname、password env、password value 和 reset password 归一化。
- 使用 `require` 和更具体的 testify 断言表达错误、参数缺失、长度、包含关系和独立属性检查，满足 `docs/TESTING.md` 的断言规范。

**Non-Goals:**

- 不修改 RBAC seed 服务、超级管理员角色常量、权限目录、角色权限绑定或用户角色绑定业务语义。
- 不新增旧 CLI alias、旧 flag、旧环境变量或旧 Make target 兼容入口。
- 不新增数据库 schema、Ent schema、Atlas migration、OpenAPI 生成物、Docker/Compose/Kubernetes/Helm 或观测资产变更。
- 不为测试制造无业务职责的生产代码分支或大型接口抽象。

## Decisions

### Decision: 使用最小依赖工厂 seam 覆盖 runner

在 `rbac.go` 中保留现有 runner 函数签名，并只抽出可替换的依赖工厂变量，让测试能够注入 fake `rbacSeedDependencies` 和 cleanup。这样可以直接覆盖 `runRBACSeedCommand`、`runAssignSuperAdminCommand` 和 `runCreateSuperAdminCommand` 的命令流程，不需要启动真实 PostgreSQL。

备选方案是用真实配置和 Ent test client 覆盖 runner，但这会把命令契约测试和数据库初始化、migration 状态、driver 行为耦合，失败定位更差。另一个备选方案是在 application 层只测 seed service，但无法覆盖 CLI runner、config path、cleanup 和参数归一化路径。

### Decision: 为依赖字段使用现有最小接口

`createSuperAdmin` 只需要 seed service、用户创建、凭据读取/更新和 password service 的少量方法。测试 seam 应尽量复用现有 application port 或在 cmd 包内定义最小接口类型，避免引入跨 feature 大接口或把测试 fake 放入生产 feature 包。

备选方案是直接依赖具体 service 类型并用真实 store 构造依赖，但这会迫使测试穿透 Ent/PostgreSQL。该变更的目标是命令契约覆盖，不是重新验证 role、user、auth 的基础设施 adapter。

### Decision: 命令表面测试和 runner 测试分层

`main_test.go` 继续覆盖 Cobra 命令名、flag 默认值和参数解析；新增或补充 `rbac_test.go` 聚焦 runner、归一化、cleanup 和 createSuperAdmin 流程。这样可以把 CLI 表面契约和业务命令执行契约分开，避免单个测试同时断言多个失败来源。

备选方案是把所有场景都通过 `newRootCommand().Execute()` 驱动，但参数错误、依赖初始化错误和 cleanup 合并错误会混在 Cobra 执行路径中，难以保持 fail-fast 断言清晰。

## Risks / Trade-offs

- [Risk] 可测试性 seam 过大可能让生产代码暴露测试专用 API。→ Mitigation：只保留 cmd 包内未导出的最小工厂变量和接口，不新增 `NewXForTest` 或跨包测试 helper。
- [Risk] fake dependency 与真实 seed service 行为不一致。→ Mitigation：runner 测试只断言命令流程、参数传递、错误传播和 cleanup；RBAC seed 业务语义继续由 role/permission application 与 store 测试覆盖。
- [Risk] `make lint` 或 `make verify` 可能被 Multica runtime 文件或其他未完成 change 的工作区 diff 阻塞。→ Mitigation：验证前暂存本次预期文件；运行结果只按本次源码、测试和 `openspec/changes/cover-rbac-cli-commands-no-compat/` 判断，忽略根 `AGENTS.md` 等 runtime 文件。

## Migration Plan

本变更无运行时 migration。实施时先创建 OpenSpec change artifacts，再补齐 cmd 包测试和最小生产 seam，运行 `go test -cover ./user-service/cmd`、`openspec validate cover-rbac-cli-commands-no-compat` 和必要 lint/verify。回滚时移除本 change artifacts、测试文件和对应最小 seam 即可，生产 RBAC 行为保持不变。

## Open Questions

- 无。
