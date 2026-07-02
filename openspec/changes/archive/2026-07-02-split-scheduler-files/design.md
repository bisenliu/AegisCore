## Context

`common/runtime/scheduler` 当前已经包含错误定义、Redis locker、cron logger、metrics、scheduler 主实现和测试文件。其中 `scheduler.go` 仍集中放置公开配置类型、构造函数、生命周期方法、任务执行状态机、分布式锁获取、自动续租、unlock、任务校验和停止状态检查。

本次 change 是 `shared-platform-primitives` 下的包内结构重构。它不改变 scheduler 的稳定 API，也不改变 `common` 与 `user-service` 的边界：scheduler 仍只提供业务中立 runtime primitive，不引入 feature 语义、Fx provider、HTTP、Ent、Redis key 业务 schema 或部署资产。

## Goals / Non-Goals

**Goals:**

- 将 scheduler 主实现拆分为职责更聚焦的文件，降低单文件维护成本。
- 保持 `package scheduler`、导出类型、导出函数、错误变量和运行时行为不变。
- 保持现有测试语义，并通过 `common/runtime/scheduler` 包测试验证拆分不改变行为。
- 在 OpenSpec 中记录 scheduler 包内结构拆分必须遵守的稳定契约。

**Non-Goals:**

- 不调整 scheduler 的公开 API、配置字段、锁策略、metrics 接口或日志消息语义。
- 不重写 Redis locker、不替换 cron 库、不新增外部依赖。
- 不新增 user-service 后台任务、不改 RBAC、auth、permission、role 或 user feature。
- 不修改数据库 schema、OpenAPI 生成物、部署清单、Prometheus/Grafana 资产或运行时配置格式。

## Decisions

- 按职责拆分 `scheduler.go`，而不是抽取新 package。
  备选方案是新增 `internal` 子包或多个子 package，但这会增加包间导出符号和测试复杂度。保持单一 `package scheduler` 可以在不改变调用方导入路径的前提下改善可维护性。

- 保留当前导出类型所在包和符号名称。
  备选方案是重命名配置类型或新增更细粒度类型，但这会影响调用方和主规格行为。本次只处理文件结构，不改变 `Config`、`JobConfig`、`LockPolicy`、`Scheduler`、`New` 等稳定入口。

- 建议拆分为类型定义、生命周期、执行流程、锁续租和校验相关文件。
  备选方案是按公开与私有符号二分，但任务执行和锁续租仍会混杂。按职责拆分更适合后续定位 local overlap、global concurrency、lock acquire、auto renew 和 validation 的变更。

- 保留现有测试文件组织，必要时只补充针对拆分风险的测试。
  备选方案是同步拆分测试文件，但这不是实现行为保持所必需的工作。优先避免扩大重构范围，只有当测试可读性明显受益时才拆分测试。

## Risks / Trade-offs

- [Risk] 拆分过程中移动代码可能遗漏 import、复制错误或改变初始化顺序 → Mitigation：保持函数体最小改动，优先移动而非改写，并运行 `go test ./runtime/scheduler` 或 common 模块对应测试。
- [Risk] 重构可能意外改变导出 API 或错误语义 → Mitigation：不改公开类型、常量、错误变量和函数签名，并通过现有 scheduler/lock 测试覆盖行为。
- [Risk] 文件数量增加后命名不一致反而降低可读性 → Mitigation：使用稳定职责命名，例如 `types.go`、`lifecycle.go`、`executor.go`、`renew.go`、`validation.go`，避免 `utils.go`、`helpers.go` 等兜底文件名。
- [Risk] specs 把一次性重构写成独立 capability → Mitigation：将 delta 归入 `shared-platform-primitives`，只增加包内结构稳定契约，不新增 `runtime-scheduler` 主规格。

## Migration Plan

1. 在 `common/runtime/scheduler` 内新增职责文件，并从 `scheduler.go` 移动对应类型和函数。
2. 保持所有文件使用 `package scheduler`，避免新导入路径和跨包依赖。
3. 运行 scheduler 包测试和必要的 common 模块测试。
4. 如发现行为差异，回滚对应文件移动或恢复函数体到原实现；该变更不涉及数据迁移或部署回滚。

## Open Questions

- 无。当前输入已明确目标为 `common/runtime/scheduler/` 拆分文件，且不要求行为变更。
