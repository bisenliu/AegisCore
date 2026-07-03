## ADDED Requirements

### Requirement: CORS middleware 回归测试不改变观测链路

系统 MUST 在补齐 `common/http/middleware` 中 CORS 默认入口测试时保持既有 runtime observability middleware 行为不变。测试补齐 MUST 限定在当前 CORS 默认策略与自定义策略稳定字段，不得修改 request ID、logging、metrics、tracing、recovery、pprof、OpenAPI 路由或 user-service 运行时 middleware 挂载策略。

#### Scenario: 观测 middleware 行为保持不变

- **WHEN** 为 `common/http/middleware.CORS()` 或 `CORSWithOptions` 增加测试覆盖
- **THEN** 系统 MUST NOT 修改 request ID、logging、metrics、tracing、recovery、pprof 或 OpenAPI 相关生产代码
- **AND** 系统 MUST NOT 改变这些 middleware 的 HTTP status、header、日志字段、metrics label 或 tracing span 语义

#### Scenario: 服务挂载策略保持不变

- **WHEN** CORS 默认入口测试补齐完成
- **THEN** user-service HTTP router 的 CORS 挂载策略 MUST 保持不变
- **AND** 本次 change MUST NOT 新增、删除或移动 user-service 运行时 CORS middleware 挂载点
