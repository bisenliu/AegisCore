## ADDED Requirements

### Requirement: Redis 命令 tracing

系统 MUST 为共享 datastore 创建的 go-redis client 安装 OpenTelemetry tracing hook，使 Redis 命令在调用方 context 包含有效 trace 时产生 Redis client span。该 tracing MUST 不改变 Redis key schema、命令结果、连接生命周期、启动 ping、Redis PING metrics 或业务缓存语义。

#### Scenario: Redis 命令产生 span

- **WHEN** 服务在有效 OpenTelemetry trace context 中通过共享 datastore Redis client 执行 Redis 命令
- **THEN** 系统 MUST 产生 Redis client span 并关联到当前 trace
- **AND** span MUST 使用 OpenTelemetry Redis instrumentation 的低基数属性

#### Scenario: Redis tracing 禁止敏感字段

- **WHEN** 系统记录 Redis command span 或 span event
- **THEN** span 属性 MUST NOT 暴露 Redis key、token、用户 ID、角色 ID、权限 ID、原始错误、密码或连接 DSN
- **AND** metrics 标签 MUST 继续遵守低基数约束

#### Scenario: Redis tracing 不改变禁用行为

- **WHEN** tracing provider 未启用或使用 no-op tracer provider
- **THEN** Redis 命令 MUST 保持原有执行结果和错误语义
- **AND** 系统 MUST NOT 因 tracing 禁用跳过 Redis 命令、启动 ping 或 Redis metrics 探测

### Requirement: Ent 查询观测

系统 MUST 为 user-service Ent query 导出 OpenTelemetry query span、query latency histogram 和 query error counter。Ent query 观测 MUST 位于服务级 Ent client/provider 边界或服务级观测代码中，不得手写修改 `user-service/ent/` 生成代码，不得把 user-service Ent entity 语义放入 `common/runtime/datastore`。

#### Scenario: Ent 查询产生 span

- **WHEN** 服务在有效 OpenTelemetry trace context 中执行 Ent query
- **THEN** 系统 MUST 产生 Ent query span 并关联到当前 trace
- **AND** span 属性 MUST 使用低基数 query/entity 信息
- **AND** span MUST NOT 记录 raw SQL、SQL 参数、用户 ID、角色 ID、权限 ID、token、DSN 或原始错误文本

#### Scenario: Ent 查询 latency 指标

- **WHEN** Ent query 执行完成
- **THEN** 系统 MUST 将本次 query 耗时写入 Ent query latency histogram
- **AND** histogram 标签 MUST 使用稳定低基数 entity/query/result 枚举
- **AND** histogram 标签 MUST NOT 包含 raw SQL、SQL 参数、用户 ID、角色 ID、权限 ID、trace/span ID 或原始错误

#### Scenario: Ent 查询错误指标

- **WHEN** Ent query 返回错误
- **THEN** 系统 MUST 增加 Ent query error counter
- **AND** error counter 标签 MUST 使用稳定低基数 entity/query 枚举
- **AND** 系统 MUST 保持原始 query error 返回语义不变

#### Scenario: Ent 观测不修改数据库契约

- **WHEN** 系统新增 Ent query tracing 和 metrics
- **THEN** 系统 MUST NOT 修改 Ent schema、Atlas migration、SQL 表结构、索引、OpenAPI 或 HTTP API
- **AND** SQL 连接池 metrics 的 metric family、label key、label value 和数值语义 MUST 保持不变
