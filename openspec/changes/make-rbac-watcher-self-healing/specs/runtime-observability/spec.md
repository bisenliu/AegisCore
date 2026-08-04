## ADDED Requirements

### Requirement: RBAC watcher 新鲜度健康与自恢复观测

user-service MUST 通过 permission application 的结构化只读 watcher status 判定 RBAC watcher 健康，并以最后一次成功 PostgreSQL revision 权威校准时间计算 staleness。公共健康响应、metrics、alerts 和 dashboard MUST 区分 stopped、reconnecting、recovered 与 stale 状态，MUST NOT 继续以任意历史 `LastError` 作为永久失败条件。

#### Scenario: 首次校准后进入就绪

- **WHEN** watcher 已运行但尚未成功完成一次 PostgreSQL revision 权威校准
- **THEN** watcher startup 和 readiness 检查 MUST 返回 unavailable 且使用稳定、不含底层错误的定位信息
- **WHEN** 首次权威校准成功且 Casbin projection ready
- **THEN** watcher startup 和 readiness 检查 MUST 恢复 available，并以该成功时间开始计算 staleness

#### Scenario: 新鲜窗口内订阅重连不制造粘滞失败

- **WHEN** watcher 正在运行且最后权威校准年龄不大于 `max_staleness`，但 subscription state 为 `reconnecting` 或保留历史失败时间
- **THEN** watcher 自身 readiness 检查 MUST 保持 available，订阅降级 MUST 通过结构化状态、metrics 和日志保持可见
- **AND** 独立 Redis health checker MAY 因 Redis 整体不可用使聚合 readiness 失败，但 watcher 检查 MUST NOT 因已恢复的历史错误永久失败

#### Scenario: 停止、从未校准或校准过期时拒绝流量

- **WHEN** watcher 未运行、循环意外退出、从未成功权威校准，或当前时间减去最后权威校准成功时间大于 `max_staleness`
- **THEN** watcher readiness MUST 返回 unavailable，且 `/readyz` MUST 返回 `503`
- **AND** 健康响应 MUST 只返回稳定的 stopped、not synchronized 或 stale 定位信息，MUST NOT 暴露原始 Redis/PostgreSQL 错误、地址、key、SQL、stacktrace 或 secret
- **AND** `/livez` MUST 继续只表达进程存活，不得因 watcher stale 而失败

#### Scenario: watcher 专用低基数指标

- **WHEN** metrics provider 启用并采集 watcher 状态
- **THEN** 系统 MUST 暴露 watcher running、subscription connected、最后订阅成功 timestamp、最后权威校准成功 timestamp、当前 reconcile staleness 和重连尝试计数
- **AND** 指标 label MUST 只使用固定 state、result 和 reason 枚举，MUST NOT 包含原始错误、Redis key、revision、event、user、role、permission 或其他高基数字段
- **AND** 系统 MUST 停止为 watcher 输出或查询 `aegiscore_runtime_component_running{resource="rbac_policy_watcher"}` 与 `aegiscore_runtime_component_last_error{resource="rbac_policy_watcher"}`，MUST NOT 双写旧指标

#### Scenario: 告警与 dashboard 表达当前风险

- **WHEN** watcher 停止或 reconcile staleness 持续超过配置预算
- **THEN** Prometheus MUST 产生可定位到实例的 critical 告警，Grafana MUST 展示 running、subscription state、最后校准成功时间和 staleness
- **WHEN** watcher 持续重连但权威校准仍在新鲜窗口内
- **THEN** Prometheus MUST 将其表达为 subscription degraded warning，MUST NOT 将单次历史错误持续表达为 watcher 不健康
- **AND** Compose dashboard MUST 由通用 dashboard 资产生成并通过 drift 检查保持一致

#### Scenario: 恢复状态可观察

- **WHEN** 初始订阅失败、Receive 错误或 revision 校准失败随后恢复
- **THEN** 当前错误和健康状态 MUST 在对应成功动作后恢复，最后成功 timestamp MUST 推进，staleness MUST 回落
- **AND** 失败与恢复日志 MUST 使用英文 message 和稳定 `snake_case` 字段，公共健康响应和 metrics label MUST NOT 包含底层 cause
