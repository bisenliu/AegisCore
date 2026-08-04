## Why

RBAC policy watcher 在初始订阅瞬时失败后会永久保留陈旧错误，订阅 channel 非预期关闭时还会同时终止 Pub/Sub 消费和数据库 revision 周期补偿，导致已恢复实例持续未就绪或永久失去副本同步能力。该缺陷会把短暂基础设施故障放大为持续摘流和授权投影不一致风险，必须在上线前建立可恢复、可诊断的 watcher 状态机。

## What Changes

- 为 RBAC policy watcher 建立单一 supervisor：初始订阅失败或 channel 关闭后，以带抖动的有界指数退避持续重建订阅，直至成功或服务停止。
- 将数据库 revision 周期补偿与 Pub/Sub 订阅可用性解耦；订阅重建期间仍持续以 PostgreSQL latest policy revision 校准本地 Casbin projection。
- 记录订阅成功、权威 revision 校准成功、当前故障和最近故障时间；对应恢复动作成功后清除当前错误，不再让历史错误永久影响健康状态。
- 以最后一次成功权威校准时间和允许的最大 staleness 判定 watcher readiness/startup，并暴露重连、订阅状态、最后成功时间和 staleness 指标。
- 新增 watcher 检查周期、订阅超时、重连退避和最大 staleness 的服务私有配置、校验与部署配置基线。
- **BREAKING**：以结构化 `Status()` 快照替换 watcher 的 `Running()`/`LastError()` 状态接口；删除 watcher 对通用粘滞 `component_last_error` 指标和旧健康判断的依赖，不保留旧接口、旧指标或旧配置别名。
- 增加初始订阅失败、持续断连、channel 关闭、自动恢复、staleness 边界、停止取消和 goroutine 清理的确定性故障注入测试。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：明确 watcher 的自恢复订阅、独立周期补偿、结构化当前状态和无泄漏生命周期要求。
- `runtime-observability`：将 watcher 健康语义改为基于运行状态与权威校准 staleness，并定义对应指标、告警和 dashboard 行为。

## Impact

- 代码：`user-service/internal/features/permission/` 的 watcher、application status port、Fx 构造与生命周期，以及 `user-service/internal/config/` 的 RBAC 服务私有配置。
- 观测：`user-service/internal/providers/observability/`、`deployments/observability/`、Compose Grafana 生成物和相关 runbook/测试。
- 部署：Nacos 配置基线及其配置测试需要新增 watcher 参数；Kubernetes/Helm probe 路径和阈值本身不变。
- 契约：内部 Go 状态接口和 Prometheus watcher 指标发生破坏性替换；HTTP 业务 API、公开错误契约、数据库 schema、Atlas migration 和 OpenAPI 生成物不变。
- 安全：恢复期间继续使用 PostgreSQL revision 和 Casbin projection 的 fail-closed 语义，Redis Pub/Sub 仍只作为非权威唤醒 hint。
