## Context

RBAC policy sync 当前由在线写入实例先执行本地 reload 或缓存失效，再通过 Redis policy version 与 Pub/Sub 通知其他副本。非写入副本使用 Pub/Sub 作为快速路径，并通过周期性版本补偿发现漏消息或重启期间错过的版本。授权热路径使用本地 Casbin enforcer 与本地用户角色解析结果，不在每次请求中读取 Redis version。

现有观测覆盖 policy sync 操作计数、version mismatch 计数、watcher 运行状态、watcher last error 和 Casbin reload 成败，但缺少当前副本与 Redis 最新 policy version 的差值指标。因此运维只能知道曾经发生过 mismatch，不能判断落后是否仍在持续，也无法把“权限变更最终生效延迟”落到 SLO 与告警。

## Goals / Non-Goals

**Goals:**

- 为 RBAC 在线写成功后的多副本最终收敛定义 30 秒 SLO。
- 新增 `aegiscore_user_service_rbac_policy_reload_lag` gauge，表示 `max(redis_latest_policy_version - local_applied_policy_version, 0)`。
- 在 Redis watcher 成功读取 Redis policy version、处理 Pub/Sub payload 和成功应用远端 version 后更新 lag 指标。
- 在 Grafana dashboard、Compose provisioning dashboard、Prometheus alert 和 runbook 中展示并告警持续 lag。
- 保持指标 label 低基数，不包含用户、角色、权限、Redis key、raw path 或原始错误。

**Non-Goals:**

- 不改变 HTTP API、OpenAPI、数据库 schema、Redis key schema 或 Casbin policy 数据模型。
- 不在授权请求热路径读取 Redis version，不引入强一致授权检查。
- 不保留旧指标名称、双写兼容分支或兼容 PromQL。
- 不引入 MQ、持久消息队列、eventbus、outbox 或新的外部依赖。

## Decisions

- lag 指标归属 permission feature metrics：`aegiscore_user_service_rbac_policy_reload_lag` 是 RBAC 业务语义，MUST 留在 `user-service/internal/features/permission/`，不得下沉到 `common/runtime/observability/metrics`。备选方案是使用 common runtime component 指标表达 lag，但该方案会把 user-service RBAC policy version 语义泄露到 common，故不采用。
- lag 使用单值 gauge，不增加业务标签：Prometheus 已通过 provider 注入稳定 `service`、`environment`、`instance` 等通用标签，业务 recorder 不新增 user、role、permission、source、reason 等标签。备选方案是按 `source` 标注 Pub/Sub 或 version check，但 lag 是当前状态而非事件来源，额外标签会造成解释歧义，故不采用。
- lag 定义为 `max(remote - local, 0)`：Redis version 是跨实例单调递增的最新版本，本地 tracker 表示本实例已成功应用版本。负值只可能来自局部初始化或异常状态，统一归零避免误报。备选方案是暴露 local 与 remote 两个 gauge，但会增加 dashboard 和 alert 的 PromQL 复杂度，故不采用。
- watcher 只在可观测点更新 lag，不改变同步控制流：`CheckVersion` 成功读取 Redis 后记录准确 lag；`HandlePayload` 和 `applyIfNewer` 可基于 payload version 记录临时 lag，成功应用后归零到该版本；Redis 读取失败不重置 lag。备选方案是在每次授权请求前校验 lag，但违反现有热路径约束，故不采用。
- Prometheus alert 使用持续 lag 触发：任一实例 `aegiscore_user_service_rbac_policy_reload_lag > 0` 持续 30 秒即告警，阈值与 RBAC 最终生效 SLO 对齐。备选方案是仅依赖 mismatch counter 增速告警，但 counter 不能表达当前是否仍落后，故不采用。

## Risks / Trade-offs

- Redis `CurrentVersion` 失败时 lag gauge 可能保留旧值 → 继续通过现有 `WatcherCheckFailed` counter、Redis up 指标和 watcher 状态共同定位，不在未知状态下把 lag 清零。
- 副本启动后 `VersionTracker` 初始为 0，首次 version check 前可能尚未暴露准确 lag → watcher 启动后的首次补偿检查会校准；启动 readiness 仍由 Casbin policy reload 状态控制。
- Pub/Sub payload 只携带消息版本，不一定等于 Redis 最新版本 → payload 路径只提供近似即时观测，准确值由周期性 `CheckVersion` 负责校准。
- 30 秒 SLO 依赖 Redis 可用和 watcher 正常运行 → Redis 不可用、watcher stopped 或 reload failed 继续由现有 critical alert 覆盖，lag alert 负责补充“仍在运行但未收敛”的窗口。

## Migration Plan

- 实现 permission metrics interface、no-op 生成物和 Prometheus gauge 注册。
- 在 Redis watcher 的 version check、payload 处理和 apply 成功路径记录 lag。
- 更新单元测试覆盖 lag 计算、Redis 读取失败不清零、reload 成功后 lag 收敛。
- 更新 Grafana dashboard 源文件与 Compose provisioning dashboard，新增 RBAC policy reload lag 面板。
- 更新 Prometheus alert，新增持续 lag 告警并指向 runbook。
- 更新 runbook，明确 RBAC policy reload lag 的含义、影响和排障步骤。
- 回滚时随应用版本回滚代码和观测资产；该变更不涉及数据库、Redis key 或 API migration。

## Verification

- 运行 permission/RBAC 相关 Go 测试，覆盖 watcher lag 记录和 metrics collector。
- 运行 `make user-service-architecture-lint` 校验架构边界。
- 运行 `make compose-dashboard-check` 校验 dashboard 源与 provisioning 生成物一致。
- 合并前运行 `make lint` 和 `make verify`。
