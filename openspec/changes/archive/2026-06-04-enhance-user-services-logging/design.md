## Context

`user-services` 已通过 `common/logger`、`common/middleware.TraceID` 和 `common/middleware.RequestLogger` 获得 Zap 结构化日志、`X-Trace-ID` 请求链路关联能力和 HTTP 请求完成日志，日志字段 `trace-id` 会随 request context 传播。当前用户、认证和会话流程已有少量 `logger.Info` 与 `logger.Error`，但部分关键异常分支没有日志，部分系统错误缺少用户、会话或请求参数上下文；`RequestLogger` 也将所有请求完成日志统一记录为 info，且缺少认证用户标识，导致 4xx/5xx 请求和匿名/认证请求的检索、告警维度不足。

本变更跨越 `service`、`controller`、`repository/redis` 和共享 logger 使用约定，但不改变 HTTP API、响应信封、数据库 schema、Redis key 或认证 token 格式。实现应遵守现有分层：controller 处理 HTTP 绑定与响应，service 记录业务流程与错误归因，repository 保留数据访问错误上下文。

## Goals / Non-Goals

**Goals:**

- 补充用户服务缺失日志，确保错误日志具备 `trace-id`、错误原因、必要业务标识和堆栈。
- 优化 `common/middleware.RequestLogger`，根据响应状态选择 info、warn 或 error，并在请求日志中加入 `user_id`，缺失时使用 `anonymous`。
- 为用户创建、用户查询、用户列表、登录、强制改密、刷新 token、登出和 Redis 会话仓储补充关键流程与异常分支日志。
- 明确错误级别：系统故障使用 error，安全或业务拒绝使用 warn，可预期成功路径使用 info，低价值细节使用 debug 或不记录。
- 建立日志脱敏约束，避免密码、token、password hash、Authorization header 等敏感数据进入日志。
- 保持代码风格与现有 Zap、`common/logger` context API 一致。

**Non-Goals:**

- 不引入新的日志库、链路追踪系统、指标系统或告警平台 SDK。
- 不改变 HTTP 响应格式、错误码、状态码或业务行为。
- 不修改 Ent schema、Atlas migration、Redis key 格式或 token claims。
- 不记录完整请求 body 或原始凭证用于审计。

## Decisions

### Decision: 复用 `common/logger`，不新增用户服务私有日志框架

用户服务日志必须继续通过 `common/logger` context API 输出，以自动携带 `trace-id` 并保持全仓库 Zap 编码、时间戳、level 和 caller 格式一致。用户服务只在现有调用点补充缺失日志和必要 Zap 字段，不新增 `user-services` 私有日志框架或统一字段 helper。

替代方案是新增 `internal/logging` 完整封装 logger 初始化和 writer。该方案会与 `shared-infrastructure` 重复，且容易导致字段格式分裂，因此不采用。

### Decision: 只补充必要业务上下文，不强制引入额外通用字段

日志消息保持现有风格，检索维度放入 Zap 字段。用户服务日志优先补充已有业务字段，例如 `user_id`、`username`、`session_id`、`page`、`page_size`、`status` 和错误对象；不强制增加 `module`、`method`、`event`、`error_code`、`error_kind` 或 `reason` 等额外字段。

替代方案是引入用户服务专用日志字段 helper 并为所有日志追加 module/method/event/error_code 等字段。该方案会增加本次变更范围和调用复杂度，因此不采用。

### Decision: 在 service 层记录业务失败，在 repository 层保留数据访问错误上下文

service 层最了解业务语义，负责将领域错误、认证拒绝和业务上下文字段记录清楚。repository 层保留 Redis/PostgreSQL 操作、用户/会话标识和底层错误上下文，但避免记录原始 SQL、完整 Redis payload 或 token。

替代方案是在 controller 统一记录所有错误。该方案会丢失 service 内部分支原因，也容易重复记录同一错误，因此只在绑定/校验失败等 HTTP 边界场景由 controller 或 common ginvalidation 记录。

### Decision: 错误级别按可操作性划分

系统异常、依赖失败、序列化失败、token 签发失败和未知错误使用 error，并携带堆栈。认证失败、token 无效、会话缺失、状态不允许、用户不存在和唯一冲突等可预期业务拒绝使用 warn 或不重复记录。正常业务入口和成功结果只记录 info，避免过多噪音。

替代方案是所有失败都使用 error。该方案会放大认证失败和校验失败等可预期事件，降低告警信噪比，因此不采用。

### Decision: HTTP 请求完成日志按状态码选择级别并记录用户标识

`RequestLogger` 应继续位于 `common/middleware`，作为共享 HTTP 请求日志能力。2xx/3xx 请求完成日志使用 info，4xx 客户端或业务拒绝使用 warn，5xx 服务端错误使用 error。请求日志应从认证 context 获取 `user_id`；若当前请求未认证、认证中间件未写入或上下文缺失，则统一记录 `user_id=anonymous`，避免字段缺失影响检索。

替代方案是在各 controller 中单独记录请求日志。该方案会重复记录请求完成事件，并破坏共享中间件统一字段，因此不采用。

### Decision: 默认只记录安全参数摘要

允许记录 UUID、用户名、分页参数、状态枚举、session id 和 token version 等排查字段。禁止记录密码、新密码、password hash、access token、refresh token、password-change token、完整 Authorization header、Redis session JSON payload 和数据库连接凭证。必要时仅记录 token/session 是否存在、长度区间或解析失败原因分类。

替代方案是记录完整请求参数。该方案存在凭证泄露风险，因此不采用。

## Risks / Trade-offs

- [Risk] 补充 warn/info/error 日志后日志量上升。→ 仅记录关键流程和异常分支，HTTP 请求完成日志保持单条中间件日志，低价值细节使用 debug 或不记录。
- [Risk] 同一错误在 service 和 repository 重复记录。→ repository 优先包装错误上下文，service 在对外映射处记录业务语义；只有依赖层必须观测的 Redis 操作失败才在 repository 记录。
- [Risk] 日志字段过度扩展增加维护成本。→ 仅补充已有业务上下文字段，不引入新的用户服务日志 helper 或额外通用字段体系。
- [Risk] 开发者误把敏感字段加入日志。→ 在 specs 和 tasks 中明确禁止字段，并补充测试或审查项。
- [Risk] 堆栈过多增加日志体积。→ 仅 error 级系统异常携带 `logger.StackTrace()`，warn 级业务拒绝不携带堆栈。

## Migration Plan

实现可作为兼容性增强滚动发布。发布后现有日志字段仍可继续存在，但新增统一字段将成为推荐检索入口。回滚时不会影响 API、数据库或 Redis 数据；仅减少新增日志字段和日志分支。

## Open Questions

- 是否需要在后续独立变更中为日志字段建立 lint 或测试 helper，以防止敏感字段回归。
- 是否需要在后续独立变更中将业务错误码与告警规则映射到外部监控平台。
