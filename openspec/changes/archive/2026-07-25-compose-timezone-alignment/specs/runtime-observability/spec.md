## ADDED Requirements

### Requirement: 本地观测时间展示边界

本地观测组件 MUST 区分容器进程日志时区、浏览器展示时区与基于 Unix epoch 的 telemetry 绝对时间。Jaeger、Prometheus 和 Grafana 的进程日志 MUST 可定位到 `Asia/Shanghai` 或 `+08:00`，但 OpenTelemetry span 与 Prometheus sample 的存储语义 MUST 保持不变。

#### Scenario: Grafana 默认展示时区

- **WHEN** Compose provisioning 加载 user-service Grafana dashboard
- **THEN** dashboard MUST 默认使用 `Asia/Shanghai` 展示时间
- **AND** 通用 dashboard 源文件与 Compose provisioning 副本 MUST 通过现有生成脚本保持一致

#### Scenario: Prometheus Web UI 时区边界

- **WHEN** 用户访问 Prometheus Web UI 或读取 Prometheus sample
- **THEN** 系统 MUST 保留 Prometheus 官方的 Unix time 内部语义与 UTC UI 展示行为
- **AND** Compose `TZ` MUST 只用于 Prometheus 进程本地时间与日志，不得宣称可以覆盖官方 UI 时区

#### Scenario: Jaeger UI 时区边界

- **WHEN** 用户访问 Jaeger UI 查看 trace
- **THEN** Jaeger 服务进程 MUST 能加载 `Asia/Shanghai`，trace 时间戳 MUST 继续表示 Unix epoch 绝对时间
- **AND** UI 日期 MUST 继续由访问浏览器的本地时区渲染，Compose MUST NOT 伪造不存在的服务端 UI timezone 配置
