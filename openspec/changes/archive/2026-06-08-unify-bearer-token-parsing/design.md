## Context

`common/security/auth` 已提供 Authorization header、Bearer token type、Bearer prefix 常量和 `StripBearerPrefix` helper，其中 Bearer 前缀按大小写无关方式剥离。`common/http/middleware/auth.go` 的 Gin 认证中间件虽然复用这些常量，但仍在中间件内通过 `strings.HasPrefix` 和 `strings.TrimPrefix` 解析 Authorization header，导致受保护 API 只接受大小写完全匹配的 `Bearer ` 前缀。

本变更跨越 shared credentials primitive 与 HTTP middleware 两个 common 包边界，目标是将 Bearer authorization 解析语义收敛到 `common/security/auth`，并让中间件复用同一入口。变更不涉及 JWT claims、token 签名、token version、数据库、Redis、Fx 启动依赖或响应信封结构。

## Goals / Non-Goals

**Goals:**

- 在 `common/security/auth` 中提供单一 Bearer authorization 解析函数，供中间件和后续认证入口复用。
- 统一 `Bearer ` 前缀大小写无关匹配语义，并继续 trim header 和 token 的首尾空白。
- 保持缺失 Authorization header、格式错误、空 token、非法 token 和过期 token 的现有错误分类与响应兼容。
- 为 shared auth parser 和 Gin middleware 补充测试，覆盖大小写不同的 Bearer 前缀与异常输入。

**Non-Goals:**

- 不新增 Basic、API key、cookie session 或其他认证传输方式。
- 不修改 JWT 签发、解析、issuer、audience、subject、claims 或 token version 校验语义。
- 不修改 user-services controller/service/repository 分层、路由挂载策略、配置项、数据库 schema、Redis key 或 Ent 生成代码。
- 不改变公开 API 的响应 envelope、HTTP status、业务错误码或公开认证失败文案。

## Decisions

### Decision: Bearer authorization parsing belongs in `common/security/auth`

认证传输常量和 `StripBearerPrefix` 已由 `common/security/auth` 拥有，新的解析入口应与这些 credential transport primitives 放在同一包内。这样 `common/http/middleware` 只负责 HTTP request handling、日志和响应映射，不再重复实现 token 提取规则。

Alternative considered: 在 `common/http/middleware` 内改为 `strings.EqualFold`。该方案改动更小，但会继续保留两套 Bearer 解析入口，后续 refresh/change-password 或其他认证入口仍可能产生行为分叉。

### Decision: Parser distinguishes invalid format from empty token

中间件当前分别记录 invalid authorization header format 与 empty bearer token，并都映射为 token invalid 响应。新的 shared parser 应能让调用方区分“缺少有效 Bearer 前缀”和“Bearer 前缀存在但 token 为空”，以保持日志和错误分类兼容。

Alternative considered: 只复用现有 `StripBearerPrefix` 并比较返回值。该 helper 支持“可选 Bearer 前缀”场景，无法直接表达 Authorization header 必须携带 Bearer 前缀的约束，且不便区分无前缀与空 token。

### Decision: Prefix is case-insensitive; token content remains unchanged after trimming

Bearer scheme 名称按大小写无关方式匹配，保持与现有 `StripBearerPrefix` 行为一致。提取出的 token 只去除首尾空白，不修改大小写或内部字符，避免改变 JWT 原文并影响签名校验。

Alternative considered: 严格要求 `Bearer ` 大小写完全一致。该方案延续当前中间件行为，但与 shared auth helper 的既有语义冲突，也会使不同认证入口继续不一致。

## Risks / Trade-offs

- [Risk] 接受 `bearer <token>` 等大小写不同前缀会扩大受保护 API 的兼容输入范围。→ Mitigation: 仅放宽认证 scheme 大小写，token 原文、JWT 签名、过期时间、issuer、audience、subject、identity fields 和 token version 校验全部保持不变。
- [Risk] Parser API 如果语义过于通用，可能被误用于允许 raw token 的场景。→ Mitigation: 命名和测试明确表达这是 Authorization header 的 required Bearer parser；保留 `StripBearerPrefix` 处理可选前缀场景。
- [Risk] 调整中间件解析逻辑可能改变错误日志分支。→ Mitigation: 中间件测试覆盖缺失 header、无 Bearer 前缀、空 token 和大小写不同 Bearer 前缀，确保响应分类和日志意图不变。

## Migration Plan

1. 在 `common/security/auth` 增加 Bearer authorization 解析函数及单元测试。
2. 将 `common/http/middleware/auth.go` 的 header 前缀判断和剥离替换为 shared parser。
3. 补充或调整 middleware 测试，覆盖 lowercase/uppercase Bearer 前缀仍能进入 JWT 解析并完成认证。
4. 在 `common/` 执行 `go test ./...` 验证 shared auth 与 middleware 行为。

Rollback 可通过恢复中间件使用 `strings.HasPrefix`/`strings.TrimPrefix` 并移除新 parser 调用完成；无数据迁移或配置回滚步骤。

## Open Questions

无。
