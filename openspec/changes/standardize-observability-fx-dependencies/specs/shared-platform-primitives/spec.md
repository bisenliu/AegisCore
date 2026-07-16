## ADDED Requirements

### Requirement: 共享 runtime Fx provider 输入治理

共享 runtime primitive 的 Fx provider MUST 使用能表达真实依赖语义的输入形式。依赖类型唯一且没有 `name`、`optional`、`group` 或其他 DI metadata 时，provider MUST 使用普通强类型参数完成装配；只有输入对象承载真实 DI metadata、较复杂输出映射或能显著提升多依赖构造可读性时，系统 MAY 保留 Params 容器。共享 runtime provider MUST 继续只消费 `common/runtime/config.Config` 等跨服务配置和 primitive，MUST NOT 读取 user-service 私有配置类型。

#### Scenario: 无 metadata 的共享 provider 输入
- **WHEN** 共享 runtime Fx provider 只需要唯一类型依赖且不需要 named、optional、group 或其他 DI metadata
- **THEN** provider MUST 通过普通强类型参数接收依赖
- **AND** 系统 MUST NOT 为该依赖保留只包裹字段且无额外语义的 Params 容器

#### Scenario: 保留有真实语义的 Params 容器
- **WHEN** provider 输入需要 named、optional、group、multi-result 映射、lifecycle orchestration 或显著提升复杂构造的可读性
- **THEN** 系统 MAY 使用 Params 容器表达这些真实 DI 或构造语义
- **AND** 本规则 MUST NOT 触发对其他有真实 metadata、配置裁剪、multi-result 输出或测试 seam 的 Params/adapter 的机械删除

#### Scenario: 共享配置边界保持跨服务
- **WHEN** `common/runtime/observability` 的 Fx provider 从配置构造 metrics 或 tracing provider
- **THEN** provider MUST 只消费 `common/runtime/config.Config` 中的共享 runtime 配置
- **AND** provider MUST NOT 导入、读取或依赖 user-service 私有配置类型
