## ADDED Requirements

### Requirement: Shared infrastructure naming cleanup preserves runtime behavior
共享基础设施相关命名标准化 SHALL 只修改低风险内部名称或文档表达，不得改变配置加载、Zap 日志、trace-id 边界名称、Redis provider、PostgreSQL provider 或 Ent runtime client 的行为。

#### Scenario: Shared infrastructure names are reviewed
- **WHEN** 实现审查 `common/config`、`common/infrastructure`、`common/logger`、`common/middleware`、`common/validation` 和服务侧基础设施 wiring 的命名
- **THEN** 实现 MUST 区分公共 Go API、内部参数名、文档表达和外部配置契约

#### Scenario: Runtime contracts are preserved
- **WHEN** 共享基础设施相关名称被标准化
- **THEN** YAML key、`AEGISCORE_` 环境变量覆盖、Redis/PostgreSQL 命名实例、`X-Trace-ID` header 和日志 `trace-id` 字段 MUST 保持不变
