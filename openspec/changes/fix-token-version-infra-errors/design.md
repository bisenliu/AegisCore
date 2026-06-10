## Context

受保护路由通过 `common/http/middleware/auth.go` 解析 Access Token，并在 `AuthWithTokenVersionValidator` 中调用 `TokenVersionValidator.ValidateTokenVersion` 校验 token version。当前中间件对 validator 返回的任何错误都统一调用 `response.TokenInvalid`，使 Redis token version cache 故障、PostgreSQL 回源失败或缓存回填失败等基础设施错误对外表现为 HTTP 401/token invalid。

认证服务侧的 token version 校验由 `user-services/internal/features/auth/app/sessions.go` 中的 `tokenVersionValidator` 实现，使用 `AuthSessionStore.GetCachedTokenVersion` 优先读取 Redis，缓存未命中时通过 `UserTokenVersionStore.GetTokenVersion` 回源 PostgreSQL，再尝试回填 Redis。领域内已经存在 `authdomain.ErrTokenVersionMismatch`，但中间件没有利用该错误区分安全拒绝与依赖故障。

本变更属于认证安全边界和 API 错误映射修正，不涉及 Ent schema、Atlas migration、Redis key 格式、运行时配置或 HTTP 路由变更。

## Goals / Non-Goals

**Goals:**

- 让 token version mismatch 继续按认证失败处理，返回 HTTP 401 和 token 专用业务码。
- 让 Redis/DB 等基础设施错误 fail-closed，阻止业务 handler 执行，但对外返回服务端故障响应而不是 token invalid。
- 在基础设施错误路径记录 error 级别日志，保留底层错误上下文、`trace-id` 和非敏感用户标识，便于告警和排障。
- 补充单元测试覆盖 mismatch 与 infra error 的响应差异。

**Non-Goals:**

- 不新增认证接口、错误码枚举、配置项、Redis key 或数据库字段。
- 不改变 token 签发、Refresh Token 轮转、退出登录或改密流程的主业务语义。
- 不把基础设施错误降级为放行请求；依赖不可用时仍必须 fail-closed。
- 不引入新的告警系统集成；本次只要求以 error 级别结构化日志提供告警输入。

## Decisions

### 1. 中间件按哨兵错误分类 validator 结果

`common/http/middleware.AuthWithTokenVersionValidator` 将对 `ValidateTokenVersion` 返回错误执行分类：当错误匹配 token version mismatch 语义时，继续返回 `response.TokenInvalid`；其他错误通过 `response.Fail` 或等价 helper 返回内部错误信封。

选择该方案是因为认证中间件已经是受保护路由的 HTTP 响应边界，错误分类放在这里可以最小化影响面，并保持 service/repository 层不依赖 Gin 或响应 helper。备选方案是在 validator 内直接返回 `response.Error`，但这会让 auth app 层依赖 `common/contract/response` 的 HTTP 语义，削弱分层边界。

### 2. 复用领域错误表达 token version mismatch

`user-services/internal/features/auth/domain.ErrTokenVersionMismatch` 继续作为版本不一致的领域哨兵错误。`tokenVersionValidator` 在当前版本与 claims 版本不一致时返回该错误；Redis 读取异常、PostgreSQL 查询异常、缓存回填异常不得包装成该错误或 `ErrTokenInvalid`。

选择该方案是因为仓库已有专门错误，且测试已覆盖 mismatch 语义。备选方案是新增 common 层错误类型，但会把服务内认证领域概念上移到 common，当前没有跨服务复用需求。

### 3. 基础设施错误使用内部错误响应

基础设施错误对外使用现有 `common/contract/response` 的内部错误响应，HTTP 500、业务码 `90000`、message `internal server error`。这满足 fail-closed 和不泄露依赖细节的要求，并避免为单一场景扩展响应码契约。

备选方案是新增 HTTP 503 或 dependency unavailable 业务码。该方案能更精确表达依赖抖动，但会扩大 `api-response-contract` 和调用方兼容面；在当前响应契约只有通用内部错误码的情况下，先使用 500/90000 是更小且兼容的改动。

### 4. 缓存回填失败按基础设施错误处理

token version cache miss 后，PostgreSQL 回源成功但 Redis 回填失败时，本次变更要求中间件校验路径返回基础设施错误，而不是静默放行或返回 token invalid。原因是用户级撤销依赖 token version cache 与 PostgreSQL 的一致投影；在认证边界忽略回填失败可能产生不稳定的后续校验行为。

备选方案是继续允许回源成功即放行并只记录 warn 日志。该方案可提升短期可用性，但会让缓存故障在高流量路径中持续被掩盖，不符合本次区分真实故障和 token invalid 的目标。

## Risks / Trade-offs

- [Risk] Redis 短暂故障会从原先可能回源成功放行变为 HTTP 500。→ Mitigation: 明确这是 fail-closed 安全策略，并通过 error 级日志触发运维关注。
- [Risk] 中间件位于 common 模块，不能直接 import user-services 的 `authdomain.ErrTokenVersionMismatch`。→ Mitigation: 在 common 认证中间件定义可选的 token version mismatch 分类契约，或在注入 validator 时适配为 common 可识别的错误，避免 common 反向依赖服务模块。
- [Risk] 错误包装层级可能导致 `errors.Is` 匹配失败。→ Mitigation: 保持 sentinel error 包装链，测试覆盖直接返回和 wrapped error 两种路径。
- [Risk] 将缓存回填失败改为失败响应可能暴露既有测试或调用假设。→ Mitigation: 更新单元测试，明确 cache miss、DB failure、cache backfill failure 三类结果。

## Migration Plan

1. 在 common 认证中间件中加入 token version mismatch 与基础设施错误的分类逻辑，保持未认证、token invalid、token expired 的既有路径不变。
2. 调整 auth app 的 token version resolver，使 Redis 非 cache miss 错误、PostgreSQL 错误和缓存回填错误按基础设施错误返回，只有版本不一致返回 mismatch。
3. 补充 `common/http/middleware` 与 `user-services/internal/features/auth/app` 相关单元测试。
4. 运行 `go test ./...`，分别在 `common/` 和 `user-services/` 模块验证。

回滚时可恢复中间件对 validator 错误的统一 `TokenInvalid` 映射，但该回滚会重新引入基础设施故障被伪装为认证失败的问题。

## Open Questions

无。
