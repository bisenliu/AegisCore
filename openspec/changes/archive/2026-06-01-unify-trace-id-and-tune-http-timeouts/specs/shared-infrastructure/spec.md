## ADDED Requirements

### Requirement: Use one trace identifier across HTTP and logger contexts

系统 MUST 将 HTTP trace header、Gin context trace key、Go context trace value 和 Zap 日志 trace 字段视为同一个 trace 标识在不同边界的规范表达。HTTP header MUST 保持为 `X-Trace-ID`，Gin context key MUST 保持为 `trace_id`，日志字段 MUST 保持为 `trace-id`。系统 MUST NOT 因这些边界名称不同而生成多个互不一致的 trace 标识。

#### Scenario: Request trace id is shared across contexts
- **Given** HTTP 请求包含合法 `X-Trace-ID` header
- **When** trace-id 中间件处理请求并业务代码通过 `common/logger` context API 输出日志
- **Then** Gin context 中的 `trace_id` 值 MUST 等于请求 header 中的 trace id
- **Then** Go `context.Context` 中的 trace id 值 MUST 等于请求 header 中的 trace id
- **Then** 日志字段 `trace-id` 值 MUST 等于请求 header 中的 trace id

#### Scenario: Generated trace id is shared across contexts
- **Given** HTTP 请求未提供 `X-Trace-ID` header
- **When** trace-id 中间件生成新的 trace id
- **Then** 系统 MUST 将生成值写入响应 `X-Trace-ID` header
- **Then** 系统 MUST 将同一个生成值写入 Gin context、Go `context.Context` 和日志字段 `trace-id`

#### Scenario: Unsafe inbound trace id is not logged
- **Given** HTTP 请求包含超长或未通过配置校验的 `X-Trace-ID` header
- **When** trace-id 中间件处理请求
- **Then** 系统 MUST 生成替代 trace id
- **Then** 系统 MUST NOT 将不安全的原始 header 值写入 Gin context、Go `context.Context`、响应 header 或日志字段
