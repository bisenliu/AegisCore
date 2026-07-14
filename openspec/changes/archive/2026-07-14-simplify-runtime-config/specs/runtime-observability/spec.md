## ADDED Requirements

### Requirement: HTTP 服务与观测端点

系统 MUST 使用 `server.http` 驱动 HTTP server 生命周期，metrics 路径校验 MUST 与 pprof 配置解耦，pprof MUST 默认关闭且不得通过业务 HTTP router 暴露。

#### Scenario: HTTP server 禁用

- **WHEN** `server.http.enabled=false`
- **THEN** 服务 MUST NOT 启动 HTTP listener
- **AND** 禁用状态 MUST NOT 要求 HTTP host、port 或 timeout 使用非零占位值

#### Scenario: 显式启用 pprof

- **WHEN** 运维人员通过显式诊断入口或 `PPROF_ENABLED` 启用 pprof
- **THEN** pprof MUST 使用独立 listener
- **AND** `PPROF_ADDR` 未配置时 MUST 使用 `127.0.0.1:6060`
- **AND** production 环境 MUST 拒绝非 loopback 监听地址

### Requirement: 云原生日志输出

系统 MUST 将应用日志输出到 stdout/stderr，核心 `LogConfig` MUST 只包含 `level` 和 `format`，MUST NOT 实现应用内文件拆分或轮转。

#### Scenario: 结构化日志分类

- **WHEN** 应用、HTTP、SQL 或 Redis 组件记录日志
- **THEN** 日志 MUST 使用稳定的 `logger` 和 `component` 字段区分来源
- **AND** 关联上下文 SHOULD 包含 service、env、trace_id、span_id、request_id 和 error

#### Scenario: Ent SQL 日志分级

- **WHEN** Ent 执行普通、慢查询或失败 SQL
- **THEN** 普通 SQL MUST 为 debug，慢 SQL MUST 为 warn，失败 SQL MUST 为 error
- **AND** 日志 MUST 包含 logger、component、db、operation、duration_ms 和 error

### Requirement: OTLP tracing 最小配置

系统 MUST 仅通过 enabled、sample_ratio、otlp_endpoint 和 insecure 配置 tracing，MUST NOT 暴露 exporter 选择字段。

#### Scenario: tracing 关闭

- **WHEN** tracing disabled
- **THEN** 服务 MUST NOT 初始化 OTLP exporter

#### Scenario: tracing 启用

- **WHEN** tracing enabled
- **THEN** otlp_endpoint MUST 非空
- **AND** insecure MUST 只表达 OTLP transport 是否使用明文
