## MODIFIED Requirements

### Requirement: pprof 诊断监听

系统 MUST 提供独立 pprof 诊断监听能力，默认 MUST 关闭。pprof MUST NOT 挂载到业务 HTTP router。临时诊断时，系统 MUST 支持通过显式配置启用 pprof，并 SHOULD 使用 loopback、`kubectl port-forward` 或等价受控通道访问。Compose 本地默认 MUST NOT 启用 pprof，也 MUST NOT 发布宿主 pprof 端口。

#### Scenario: Compose 默认不暴露 pprof

- **WHEN** 调用方渲染默认 Compose 配置
- **THEN** user-service 环境变量 MUST NOT 设置 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED=true`
- **AND** user-service 环境变量 MUST NOT 设置 `AEGISCORE_OBSERVABILITY_PPROF_ADDR=0.0.0.0:6060`
- **AND** user-service ports MUST NOT 包含 `6060:6060`

### Requirement: Redis tracing 命令过滤

系统 MUST 为 Redis tracing 过滤认证类敏感命令和本地健康检查命令，避免敏感认证参数进入 span，并降低无业务价值的 ping 噪声。

#### Scenario: Redis command filter 语义

- **WHEN** Redis command 为 `AUTH`
- **THEN** command filter MUST 返回 true 表示过滤该命令且不生成 span
- **WHEN** Redis command 为 `HELLO ... AUTH ...`
- **THEN** command filter MUST 返回 true 表示过滤该命令且不生成 span
- **WHEN** Redis command 为 `PING`
- **THEN** command filter MUST 返回 true 表示过滤该命令且不生成 span
- **WHEN** Redis command 为普通业务命令
- **THEN** command filter MUST 返回 false 表示允许生成 span

### Requirement: 本地 DB tracing 分层

系统 SHOULD 在本地 Compose 诊断配置中保留 Ent 实体级 tracing 和 PostgreSQL SQL/driver 级 tracing。文档 MUST 说明 Ent span 用于观察实体与操作，otelsql span 用于观察真实 SQL/driver 视角，并说明该组合会增加 trace 细节和噪声。

#### Scenario: Compose tracing 文档说明

- **WHEN** 本地 Compose 默认启用 tracing 和 Ent tracing 插件
- **THEN** README MUST 说明会同时产生 Ent 实体级 span 和 PostgreSQL SQL/driver 级 span
- **AND** README MUST 说明该配置用于本地完整链路诊断
