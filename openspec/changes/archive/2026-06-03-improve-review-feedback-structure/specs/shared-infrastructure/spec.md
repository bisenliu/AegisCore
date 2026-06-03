## ADDED Requirements

### Requirement: Structure shared infrastructure organization review feedback
系统 SHALL 在整理共享基础设施目录组织类代码评审意见时，使用中文分别给出问题说明、原因分析和建议改法，并围绕 `common/infrastructure/` 的可维护性、职责边界和后续扩展成本给出可执行建议。

#### Scenario: Explain flat infrastructure directory risk
- **WHEN** 评审意见涉及 `common/infrastructure/` 未按基础设施类型进一步拆分
- **THEN** 反馈 MUST 在问题说明中指出 Redis、PostgreSQL、MongoDB、RabbitMQ 等组件持续增加后，单目录容易出现文件过多和职责混杂
- **THEN** 反馈 MUST 在原因分析中说明平铺目录会降低定位效率、增加跨组件修改风险，并弱化共享基础设施 provider 的职责边界
- **THEN** 反馈 MUST 在建议改法中给出按基础设施类型或职责分层的组织建议

#### Scenario: Recommend maintainable infrastructure layering
- **WHEN** 反馈给出 `common/infrastructure/` 的拆分建议
- **THEN** 反馈 MUST 至少包含按基础设施类型拆分的示例，例如 `redis/`、`postgres/`、`mongo/`、`rabbitmq/`
- **THEN** 反馈 MAY 补充按职责拆分的示例，例如 `datastore/`、`messaging/`、`logging/`、`config/`
- **THEN** 反馈 MUST 明确后续重构需要保持 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例和 Fx named injection 行为不变
