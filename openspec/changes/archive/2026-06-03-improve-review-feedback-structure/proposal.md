## Why

当前代码评审反馈中包含 Go 包命名规范和共享基础设施目录组织两个问题，但表达需要进一步结构化，才能作为后续整改与评审对齐的依据。将反馈整理为清晰、专业、可执行的问题说明、原因分析和建议改法，有助于统一团队对包命名和基础设施分层的判断标准，降低后续重构沟通成本。

## What Changes

- 将 `user-services/internal/errmsg/` 相关反馈整理为围绕 Go 包命名最佳实践的可执行意见，明确避免混合缩写风格，并建议使用全小写、短小、语义明确的包名，例如 `errmsg`。
- 将 `common/infrastructure/` 相关反馈整理为围绕共享基础设施可维护性的可执行意见，说明 Redis、PostgreSQL、MongoDB、RabbitMQ 等组件持续增加后单目录聚合的风险。
- 给出按基础设施类型或职责分层组织目录的建议，使反馈能直接指导后续代码结构调整。
- 不在本 change 中承诺 API、错误码、配置项或数据模型行为变更。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `project-naming-consistency`: 补充 Go 包命名评审反馈的结构化输出要求，覆盖缩写词、全小写包名和可执行改名建议。
- `shared-infrastructure`: 补充共享基础设施目录组织评审反馈的结构化输出要求，覆盖按基础设施类型或职责拆分目录的建议。

## Impact

- 影响文档和评审输出标准，后续实现可能涉及 `user-services/internal/errmsg/` 包命名调整及引用更新。
- 影响共享基础设施代码组织建议，后续实现可能涉及 `common/infrastructure/` 下 Redis、PostgreSQL 及未来 MongoDB、RabbitMQ 等 provider 的目录分层。
- 不改变当前外部 HTTP API、响应信封、错误码、配置格式、数据库 schema 或运行时依赖行为。
