## ADDED Requirements

### Requirement: 服务级 provider 物理边界治理

系统 MUST 通过正式架构文档、能力地图和 change artifacts 明确 user-service 服务级 provider 的关注点边界。`user-service/internal/providers` 根包 MUST 仅作为服务级 provider module 汇总入口，具体 datastore、observability、security 和 transport 接线 MUST 按关注点放入对应子包，MUST NOT 在根包保留具体 provider 构造器的兼容 wrapper、alias 或兼容分支。

#### Scenario: provider 根包保持汇总职责

- **WHEN** 协作者查看 `user-service/internal/providers` 根包
- **THEN** 根包 MUST 只暴露或维护 `WiringModule`、`RuntimeModule`、`Module` 及其必要测试
- **AND** PostgreSQL、Redis、Ent、JWT、health checks、metrics、tracing、Gin、routes 和 rate limiters 的具体构造器 MUST 位于对应关注点子包

#### Scenario: provider 子包归属清晰

- **WHEN** 协作者修改 user-service datastore 接线
- **THEN** PostgreSQL、Redis、Ent client、Ent plugins、Ent SQL log、Ent metrics 和 Ent tracing 相关代码 MUST 位于 `user-service/internal/providers/datastore`
- **WHEN** 协作者修改 user-service observability 接线
- **THEN** health checks、runtime dependency metrics、metrics provider 和 tracing provider 相关接线 MUST 位于 `user-service/internal/providers/observability`
- **WHEN** 协作者修改 user-service security 接线
- **THEN** JWT service、认证 token policy 和 password service 相关接线 MUST 位于 `user-service/internal/providers/security`
- **WHEN** 协作者修改 user-service HTTP transport 接线
- **THEN** Gin mode、Gin engine、routes 和 API rate limiters 相关代码 MUST 位于 `user-service/internal/providers/transport`

#### Scenario: 文档与测试跟随 provider 边界

- **WHEN** provider 目录结构发生变化
- **THEN** `docs/ARCHITECTURE.md` 和 `docs/opsx/CAPABILITY_MAP.md` MUST 同步描述当前 provider 子包边界
- **AND** provider 测试 MUST 跟随被测关注点放入对应子包，MUST NOT 为测试便利在生产代码中新增冗余 API、全局替身或兼容分支
