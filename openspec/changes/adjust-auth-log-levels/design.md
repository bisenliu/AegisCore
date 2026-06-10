## Context

`user-authentication` 通过 `common/http/middleware/auth.go` 的 Gin 中间件校验 JWT Bearer access token，并依赖 `common/security/auth` 提供 Bearer 解析、JWT claims 校验、认证 context helper 和 token version 校验接口。当前中间件在认证失败路径中把缺少 header、格式错误、空 token、JWT 解析失败和过期 token 都记录为 Error。

这些失败多数来自外部调用方输入，不表示服务端依赖异常。正式环境中将其计入 Error 会放大错误率、干扰告警，并掩盖真正需要 Error 关注的配置或依赖问题。该变更只调整日志等级，不改变认证结果、HTTP 状态码、响应信封或 token 校验语义。

## Goals / Non-Goals

**Goals:**

- 将认证中间件中的预期认证失败从 Error 降级为 Warn 或 Info。
- 保持 token version mismatch 作为 Warn，并保留结构化字段便于审计和排查。
- 将服务端配置或依赖异常继续记录为 Error，例如 JWT secret 缺失、token version validator 依赖失败。
- 用测试覆盖日志等级，防止后续回归为 Error。

**Non-Goals:**

- 不改变 `common/security/auth` 的错误类型、JWT claim、签发或解析规则。
- 不改变 HTTP 响应 code/message/status 或 `common/contract/response.Envelope` 结构。
- 不新增配置项控制日志等级。
- 不调整登录、刷新、登出等 `user-services/internal/features/auth` 业务流程。
- 不涉及 Ent schema、Atlas migration、Redis 或 PostgreSQL 数据结构变更。

## Decisions

- 预期认证失败使用非 Error 等级。

  缺少 `Authorization` header 属于未认证访问，使用 Info 足以说明请求没有认证凭证。格式错误、空 Bearer token、JWT 无效、JWT 过期、subject 错误、必要 claim 缺失和 token version mismatch 表示调用方提供了不可接受的凭证，使用 Warn 保留安全审计价值但不污染 Error。

- 服务端异常继续使用 Error。

  `auth.ErrMissingSecret` 表示运行时 JWT 配置错误，token version validator 返回非 `ErrTokenVersionMismatch` 的错误通常表示 Redis、数据库或适配器异常，这些路径应继续记录 Error 并保持 `response.InternalError` 处理。

- 分类逻辑保留在 `common/http/middleware/auth.go`。

  Bearer 解析和 JWT 错误定义仍属于 `common/security/auth`，但日志等级是 HTTP 中间件的可观测性策略，应在消费错误的位置决定。这样避免让 security auth 包依赖 logger 或 HTTP 响应语义。

- 测试使用可观测 logger 校验日志等级。

  在 `common/http/middleware/auth_test.go` 中为代表性认证失败增加日志断言，覆盖缺 header、格式错误、过期 token、token version mismatch 和依赖异常。测试不应依赖完整日志文本以外的非必要实现细节，但应确保预期认证失败不会产生 Error。

## Risks / Trade-offs

- [Risk] 降低为 Info/Warn 后，部分已有 Error 告警不再捕获认证失败激增。
  Mitigation: 格式错误、无效 token、过期 token 和 token version mismatch 仍保留 Warn；告警应基于 Warn 计数、HTTP 401 指标或安全审计日志，而不是 Error。

- [Risk] JWT 解析错误来源多样，可能同时包含调用方错误和配置错误。
  Mitigation: 显式识别 `auth.ErrMissingSecret` 为 Error，其余 ParseToken 返回错误按认证失败处理为 Warn，并保持响应语义不变。

- [Risk] 测试若过度绑定日志消息会增加维护成本。
  Mitigation: 只断言日志等级与代表性消息，避免检查无关字段顺序或完整结构。

## Migration Plan

- 修改 `common/http/middleware/auth.go` 的认证失败日志等级。
- 补充或更新 `common/http/middleware/auth_test.go` 中的日志等级断言。
- 在 `common/` 模块运行 `go test ./...` 验证共享包测试。
- 部署无需数据迁移；回滚只需恢复日志等级实现。

## Open Questions

- 无。
