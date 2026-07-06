## Context

本次 change 只涉及 `common` 模块的 runtime primitive、测试基础设施和模块依赖维护。当前 scheduler 已拆分为多个文件，但 `runJob()` 仍集中处理本地 gate、全局 gate、分布式锁、任务上下文、续租、执行、panic recovery、cleanup 和指标记录；锁释放、续租默认超时仍以 `5 * time.Second` 分散在多个位置。PostgreSQL 测试容器 helper 的 Docker mapped port 探测间隔仍以内联 `100 * time.Millisecond` 表达。`common` 模块还存在 `GOWORK=off go mod tidy -diff` 可识别的依赖残留。

该变更不改变 HTTP API、数据库 schema、OpenAPI、部署清单、观测资产或安全边界；不向 `user-service` feature、`internal/shared` 或 `internal/integration` 移动任何代码。

## Goals / Non-Goals

**Goals:**

- 用包内私有命名常量统一表达 scheduler 锁释放和锁续租默认超时。
- 用测试 helper 私有命名常量表达 PostgreSQL 容器端口探测间隔。
- 将 `runJob()` 拆为职责清晰的私有函数，保持导出 API、指标、日志、锁和 shutdown 行为不变。
- 对 `common` 模块执行 `GOWORK=off go mod tidy`，只保留模块实际需要的依赖。
- 保持 `common` 的业务中立 runtime primitive 边界。

**Non-Goals:**

- 不新增公开配置项或公开常量。
- 不保留旧内联 fallback 或兼容 alias。
- 不更改 scheduler 的公开类型、错误变量、cron parser、锁策略、续租策略、metrics 事件或日志消息。
- 不清理 `user-service` 中由 Gin、Swagger UI 或 Prometheus 真实导入链带入的间接依赖。
- 不修改数据库 migration、OpenAPI 生成物、部署资产或业务 feature 行为。

## Decisions

1. scheduler 默认超时使用包内私有常量

   决策：在 `common/runtime/scheduler` 中定义 `defaultLockUnlockTimeout` 和 `defaultLockRenewTimeout`，并在 `executor.go`、`renew.go`、`validation.go` 统一引用。

   理由：这些默认值属于 scheduler 内部运行策略，不需要成为跨服务公开契约；集中命名能提升可读性和后续调整可追踪性。

   备选方案：把默认值暴露到 `Config` 或 `JobConfig`。未采用，因为当前需求只是移除魔法值，不需要扩大公开配置面。

2. PostgreSQL 容器探测间隔使用测试 helper 私有常量

   决策：在 `common/testing/containers/postgres.go` 中定义 Docker mapped port 探测间隔常量，并替代内联 `time.Sleep(100 * time.Millisecond)`。

   理由：该间隔仅服务测试容器 helper，不需要成为调用方配置。命名后可与同文件 readiness probe 间隔形成清晰区分。

   备选方案：新增 `PostgresOptions` 字段控制探测间隔。未采用，因为这会把内部轮询细节暴露给测试调用方，增加无必要 API。

3. `runJob()` 拆分为私有流程函数

   决策：保留 `runJob(cfg, localGate)` 入口，将执行状态收敛到私有 state，并拆出执行权获取、锁获取、任务上下文/续租准备、cleanup 和结果记录函数。

   理由：拆分后主流程仍按原顺序串联，但每个私有函数只承担一个职责，便于审查 cleanup 顺序、错误路径和 metrics 行为。

   备选方案：引入新的公开 executor 类型或接口。未采用，因为当前目标是包内维护性改进，引入公开抽象会扩大 API 面并增加测试驱动冗余生产代码风险。

4. 依赖清理只以模块级 tidy 为准

   决策：仅在 `common` 目录执行 `GOWORK=off go mod tidy` 并接受其 diff；不手工删除 `user-service` 中仍由当前导入链需要的间接依赖。

   理由：`quic-go` 和 `mongo-driver` 由 Gin 依赖链带入，`swaggo/swag` 和 `gopkg.in/yaml.v2` 由 Swagger UI/OpenAPI 链路带入，手工删除会被 tidy 恢复或破坏构建认知。

   备选方案：通过工具 module 或 `tools.go` 隔离所有点名依赖。未采用，因为这些依赖并非全部构建时工具依赖，部分属于运行时 HTTP 路由和 Gin 依赖图。

## Risks / Trade-offs

- [Risk] `runJob()` 拆分时 cleanup 顺序变化可能导致锁释放、gate 归还或 renew error 记录语义回归。→ Mitigation：保持原有顺序，新增或保留 scheduler 单元测试覆盖跳过、失败、panic、锁释放和续租失败路径。
- [Risk] 默认超时常量集中后遗漏 `validation.go` 或 `renew.go` 某处 fallback。→ Mitigation：用 `rg '5\s*\*\s*time\.Second' common/runtime/scheduler` 验证 scheduler 中不再存在目标内联默认值。
- [Risk] PostgreSQL helper 改动引入等待行为变化。→ Mitigation：仅替换同值常量，不改变启动超时、Docker 命令或 readiness probe 语义。
- [Risk] `common` tidy 清理可能移除误以为需要的依赖。→ Mitigation：使用 `GOWORK=off go mod tidy -diff` 预览，再运行 `make common-test` 和 `make lint`/`make verify` 验证。

## Migration Plan

1. 修改 `common/runtime/scheduler`，提取默认超时常量并拆分 `runJob()` 私有流程函数。
2. 修改 `common/testing/containers/postgres.go`，提取 Docker port probe interval 常量。
3. 在 `common` 目录执行 `GOWORK=off go mod tidy`，只提交 `common/go.mod` 和 `common/go.sum` 的实际 tidy 结果。
4. 运行 `go test ./runtime/scheduler ./testing/containers` 或等价相关包测试；随后运行 `make common-test`。
5. OpenSpec 和代码变更完成后，运行 `make user-service-architecture-lint`、`make lint`、`make verify`。

Rollback 方式：该变更不涉及数据迁移和外部 API；若验证失败，回退本 change 涉及的 `common` 代码和 `common/go.mod`/`common/go.sum` diff 即可。

## Open Questions

- 无。
