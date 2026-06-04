## Context

AegisCore 的 `shared-infrastructure` capability 当前通过 `common/runtime/config`、`common/runtime/logger` 和 `common/runtime/infrastructure` 提供配置加载、Zap 日志、Redis/PostgreSQL 命名实例 provider 与运行时资源名。`common/runtime/infrastructure` 同时包含 Fx provider、datastore 初始化和资源名契约，虽然现有行为稳定，但包边界过宽，调用方在 `user-services/internal/bootstrap` 中需要使用 `commoninfra` alias 来区分语义。

本设计面向 `shared-infrastructure` 与 `common-module-organization` 两个既有 capability。目标是重组 Go 包路径并明确纯逻辑与 Fx adapter 的边界，不改变服务启动、配置加载、日志、Redis/PostgreSQL lifecycle、Ent client wiring 或 HTTP API 行为。

## Goals / Non-Goals

**Goals:**

- 将 `common/runtime/infrastructure` 拆为职责明确的 runtime 包：`configfx`、`loggerfx`、`datastore`、`datastorefx`、`resources`。
- 保持 `common/runtime/config` 和 `common/runtime/logger` 作为无 Fx provider 职责的纯逻辑包。
- 将 Redis/PostgreSQL client 或连接池构造与 Fx provider/lifecycle 注册分离。
- 将运行时资源名常量放入明确的契约包，减少 provider 包承担跨资源常量职责。
- 更新用户服务 bootstrap 和测试 import，去除泛化 `commoninfra` alias。
- 通过测试证明命名 datastore provider 仍只连接显式声明的实例。

**Non-Goals:**

- 不修改 YAML key、`AEGISCORE_` 环境变量映射、Redis/PostgreSQL 命名实例名称或 Fx name tag 字面量。
- 不修改 Zap logger 初始化语义、trace-id 字段、日志 sync 错误处理或日志文件策略。
- 不修改 Ent schema、Atlas migration、数据库表结构或 user-services 的 controller/service/repository 分层。
- 不新增 Redis/PostgreSQL 之外的新基础设施能力。
- 不拆分 `common` 或 `user-services` Go module。

## Decisions

1. 拆分为纯逻辑包和 Fx adapter 包。

   `common/runtime/config` 与 `common/runtime/logger` 保持纯逻辑；新增 `configfx` 和 `loggerfx` 承担 `fx.Provide` 可用的 provider 构造与 logger lifecycle。Redis/PostgreSQL 的 client 或连接池构造进入 `datastore`，`ProvideNamedRedis`、`ProvideNamedPostgres` 及 `fx.Lifecycle` hook 进入 `datastorefx`。

   备选方案是继续保留 `infrastructure` 并只增加文档约束。该方案改动小，但无法解决 import 语义过宽和后续能力堆叠风险。当前用户已明确希望采用细粒度目录结构，因此选择拆包。

2. 使用 `resources` 承载运行时资源名常量。

   `NameUserDB`、`NameCommonDB`、`NameCacheRedis` 等跨 Redis、PostgreSQL 和 Ent wiring 的常量不属于单个 datastore provider。放入 `common/runtime/resources` 能表达其契约性质，也避免 `datastorefx` 因常量而被非 Fx 场景依赖。

   备选方案是把常量放入 `datastore`。该方案对 Redis/PostgreSQL 合理，但 Ent runtime wiring 也使用这些名称，语义上不如 `resources` 明确。

3. 保持服务侧显式声明 runtime dependencies。

   `user-services/internal/bootstrap` 继续显式声明 `cache_redis`、`user_db` 和 `common_db`，只更新 import 和 provider 调用位置。公共包不提供“一次性装配所有 Redis/PostgreSQL 实例”的 module，避免因配置中存在未声明实例而自动连接。

   备选方案是提供聚合 `datastorefx.Module` 自动创建全部配置实例。该方案简化 wiring，但会破坏现有 opt-in 依赖契约。

4. 删除或清空旧 `infrastructure` 包，而不是保留兼容 shim。

   当前仓库内调用方可一次性迁移，且没有外部发布兼容要求。保留旧包 shim 会延续泛化入口，降低拆包收益。

   如果后续发现存在外部消费者，再通过单独变更评估兼容策略。

## Risks / Trade-offs

- [Risk] Go import 路径调整可能遗漏测试或服务侧引用 → 通过 `go test ./...` 分别验证 `common` 与 `user-services` 模块。
- [Risk] 拆分过程中不小心改变 Redis/PostgreSQL ping、close 或错误上下文 → 保留并迁移现有 infrastructure 测试，覆盖缺失配置、单实例连接和 lifecycle 行为。
- [Risk] `datastore` 与 `datastorefx` 职责边界被重新混淆 → 规格要求 `datastore` 不 import Fx，Fx lifecycle 只出现在 `datastorefx`、`configfx` 或 `loggerfx` 等 adapter 包。
- [Risk] 删除旧 `infrastructure` 包会影响未来未发现的内部引用 → 使用全仓搜索和测试验证，不做兼容 shim 以保持边界清晰。
