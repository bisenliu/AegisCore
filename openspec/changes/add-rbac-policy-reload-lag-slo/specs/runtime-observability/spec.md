## MODIFIED Requirements

### Requirement: Metrics 与部署观测资产

系统 MUST 提供显式表达启用或禁用状态的非 nil Prometheus metrics provider。HTTP、runtime、scheduler、workerpool、SQL、Redis、localcache 和 feature metrics MUST 稳定、低基数且不泄露敏感数据；localcache 稳定指标契约 MUST 只覆盖请求、回源、自动驱逐和 item 容量。系统 MUST 维护 Prometheus alerts、Grafana dashboards、Compose 观测配置、生成脚本和 runbook。PostgreSQL 与 Redis 资源名 MUST 分别为 `primary_db` 和 `cache_redis`；metrics `service` label、tracing `service.name`、日志与健康响应 `service` 字段、dashboard 变量和 alert 表达式 MUST 统一使用 `aegiscore-user-service`，MUST NOT 保留旧 `aegiscore-user-services` label、查询或兼容资产。metrics Fx provider MUST 以可识别的能力名称由 composition root 显式装配。

#### Scenario: Metrics 启停、依赖与标签契约

- **WHEN** metrics 启用
- **THEN** 系统 MUST 注册配置化 endpoint 并导出已注册 collector
- **WHEN** metrics 禁用
- **THEN** 系统 MUST NOT 暴露 endpoint 或 collector，但 MUST 为正式依赖图提供非 nil no-op provider
- **AND** metrics、tracing 和 feature-local `Metrics` MUST 是非 optional 依赖，缺失时 MUST 构图失败；provider 公开名称 MUST 表达 metrics 能力语义，MUST NOT 以通用 `NewFxProvider` 作为主要入口
- **WHEN** 系统记录 metrics、健康检查或告警查询
- **THEN** label MUST NOT 包含用户、角色、权限、会话或 token ID、trace/span ID、raw path、IP、邮箱、用户名、SQL、Redis key 或原始错误
- **AND** 资源名、指标 label、健康检查名称、dashboard 和 alert MUST 一致，低基数 allowlist、HTTP label names 和 duration buckets 的顺序与数值 MUST 稳定且调用方不可修改
- **WHEN** metrics middleware 跳过 runtime endpoint 或其他配置化请求
- **THEN** 请求总数和耗时 MAY 不记录，但 in-flight gauge MUST 在请求结束后归零，MUST NOT 因删除共享 label value 破坏并发计数
- **WHEN** feature-local `Metrics` 需要空实现
- **THEN** 系统 MUST 通过统一生成入口维护匹配接口的 no-op；业务指标 MUST 留在所属 feature，`common/runtime/observability/metrics` MUST NOT 承载 user-service 业务语义

#### Scenario: 本地缓存指标契约

- **WHEN** cache 命中、未命中、loader 成功或失败
- **THEN** 系统 MUST 分别通过 `aegiscore_localcache_requests_total{cache,result="hit|miss"}` 和 `aegiscore_localcache_loads_total{cache,result="success|error"}` 导出累计值，success MUST 直接来自 `Stats.LoadSuccess`，MUST NOT 由 load 总数减去 error 推导
- **WHEN** TTL 过期或达到最大 item 数自动移除条目，或 collector 读取容量
- **THEN** 系统 MUST 通过 `aegiscore_localcache_evictions_total{cache}` 和 `aegiscore_localcache_capacity{cache}` 导出累计自动驱逐值与配置容量，显式 `Delete` 或 `Clear` MUST NOT 计入 eviction
- **AND** 标签 MUST 仅使用固定 cache 名和 result 枚举，MUST NOT 包含 raw key、身份标识或原始错误
- **AND** 系统 MUST NOT 导出或查询 `aegiscore_localcache_singleflight_total`、`aegiscore_localcache_writes_total`，MUST NOT 将 shared、double-check、set-dropped 或 admission-rejected 保留为稳定统计字段、event label 或兼容 PromQL
- **WHEN** metrics provider 禁用
- **THEN** localcache collector MUST NOT 注册，但 localcache MUST 继续维护可由 `Stats()` 读取的快照
- **AND** dashboard、alert、metrics load 校验和 runbook MUST 仅消费 requests、loads、evictions、capacity 及固定 `cache`、`result` 标签，并与源 dashboard 和 provisioning JSON 在同一变更中保持一致

#### Scenario: 依赖探测、安全指标与资产 drift

- **WHEN** metrics HTTP scrape context 被取消
- **THEN** Redis PING MUST 尽快终止；标准 `Collect` 直接调用 MUST 使用 background context 与 collector timeout，MUST NOT 声称感知 HTTP 取消
- **AND** 最小探测间隔、快照复用及 `aegiscore_redis_*` 指标契约 MUST 保持不变
- **WHEN** dashboard 展示 RBAC Enforce 性能
- **THEN** MUST 使用低基数 histogram 展示 P95 和 P99，并同步到源码与 Compose provisioning dashboard
- **WHEN** 一次性会话消费、重复消费拒绝、撤销投影或补偿失败
- **THEN** 系统 MUST 记录对应低基数指标，alert 与 metrics load 校验 MUST 覆盖影响安全撤销的信号并指向稳定 runbook
- **WHEN** Prometheus rules、Grafana dashboard、Compose scrape config、观测文档、dashboard source 或生成逻辑变化
- **THEN** 查询、变量、静态 label、告警 label、dashboard UID、rule group、job name 和示例 MUST 使用单数服务名，生成脚本 MUST 更新 provisioning JSON
- **AND** `make compose-dashboard-check` 或等价校验 MUST 在生成物 drift 时失败

#### Scenario: RBAC policy reload lag 观测与告警

- **WHEN** permission feature 记录 RBAC policy sync 状态
- **THEN** 系统 MUST 导出 `aegiscore_user_service_rbac_policy_reload_lag` gauge 表示当前实例 Redis 最新 policy version 与本地已应用 policy version 的非负差值
- **AND** 该指标 MUST 使用 provider 注入的稳定 service、environment 和 instance 标签，业务 recorder MUST NOT 增加用户、角色、权限、Redis key、raw path、source、reason 或原始错误标签
- **WHEN** 任一 user-service 实例的 `aegiscore_user_service_rbac_policy_reload_lag` 大于 `0` 持续 30 秒
- **THEN** Prometheus alert MUST 触发 RBAC policy reload lag 告警，并指向稳定 runbook
- **WHEN** Grafana dashboard 展示 RBAC policy sync 状态
- **THEN** dashboard MUST 展示每个实例当前 policy reload lag，并与 Compose provisioning dashboard 保持一致
