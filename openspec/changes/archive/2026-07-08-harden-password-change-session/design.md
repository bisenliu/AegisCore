## Context

当前强制改密流程由 `Login` 在用户状态要求改密时签发 subject 为 `password_change` 的受限 JWT。该 token 携带 `jti`、`session_id`、`user_id` 和 `token_version`，但 `session_id` 不对应服务端一次性会话，`ChangePassword` 只校验 JWT 与当前 `token_version`。同时，password-change token TTL 复用 `access_token_ttl`，当前示例配置为 15 分钟。

改密成功后，凭据更新会递增 PostgreSQL `users.token_version`，随后 `RevokeUserSessionsAtVersion` 刷新 Redis token version 投影、删除 refresh session 并失效本地缓存。该撤销链路失败时，当前 use case 只记录日志并仍返回 `Changed: true`，调用方无法区分完全成功与安全撤销部分失败。

本次变更是安全收紧，不保留旧行为兼容。实现必须保持 feature-first 边界：强制改密一次性会话属于 `user-service/internal/features/auth`，Redis key schema 和 Lua 脚本不得放入 `common`；JWT 通用 claims 仍保留在 `common/security/auth`；配置结构和校验位于 `common/runtime/config`。

## Goals / Non-Goals

**Goals:**

- 为 `password_change` token 引入独立短 TTL 配置 `auth.jwt.password_change_token_ttl`，默认 5 分钟。
- 为强制改密流程引入 Redis 一次性 password-change session，绑定 `jti`、`session_id`、`user_id` 和 `token_version`。
- 在 `ChangePassword` 更新凭据前原子消费 password-change session，使复用、过期、撤销、claims 不一致和并发消费全部失败。
- 使用旧 `token_version` 和强制改密状态作为凭据更新条件，避免同一 token 并发双写。
- 改密成功后撤销投影失败时不再返回普通成功，必须返回可观察的安全撤销未完成错误，并记录指标。
- 补充单元测试、Redis 脚本测试、HTTP 映射测试、OpenAPI 生成物和 Prometheus 告警。

**Non-Goals:**

- 不新增普通已登录用户修改密码能力。
- 不引入数据库 schema 或 Atlas migration。
- 不引入通用 eventbus、MQ、outbox 或跨 feature retry framework。
- 不兼容旧配置缺省语义之外的长 TTL 行为；旧的 `access_token_ttl` 不再影响 password-change token TTL。

## Decisions

1. 使用独立 TTL 配置而不是继续复用 access TTL。

   理由：强制改密 token 是一次性敏感凭据，生命周期应短于普通 access token，并能独立调优。默认值使用 5 分钟，兼顾用户完成表单的可用性和截获窗口控制。

   备选方案：继续使用 access TTL 并只增加 Redis 一次性消费。该方案仍会让截获 token 在较长时间内具备发起尝试的能力，不满足安全收紧目标。

2. 新增 auth feature 内的 `PasswordChangeSessionStore`，不复用 refresh session store。

   理由：refresh session 是长期续期会话，password-change session 是一次性短凭据，两者 TTL、索引、消费语义和清理策略不同。单独 port 能避免把短期一次性语义污染到 refresh session 生命周期。

   备选方案：把 password-change session 塞进现有 `RefreshSessionStore`。该方案会混淆普通 session 上限裁剪、批量删除和一次性消费语义，容易导致误用。

3. Redis 消费使用单 Lua 脚本执行校验和删除。

   理由：`GET` 后再 `DEL` 无法抵御并发复用。Lua 脚本可在 Redis 内原子校验 `user_id`、`session_id`、`jti`、`token_version` 并删除 key，保证只有一个请求成功消费。

   备选方案：使用 `SETNX` 标记已消费。该方案需要额外 key 和 TTL 协调，仍要处理原 session 与 consumed marker 的一致性，复杂度更高。

4. 凭据更新使用旧 `token_version` 和强制改密状态条件。

   理由：Redis 一次性消费解决同一个服务端 session 的并发复用，但仍需要数据库层保护状态机。条件更新确保只有仍处于强制改密状态且版本等于 token claims 的用户能完成更新。

   备选方案：沿用先查询状态再无条件更新。该方案在并发下可能两个请求都基于旧状态完成更新，最后提交者覆盖密码。

5. 改密撤销投影失败返回错误，不返回 `Changed: true`。

   理由：密码已更新但 token version 投影或 refresh session 删除失败属于安全敏感部分成功。对调用方呈现完全成功会隐藏旧 token/session 的失效窗口。返回 `ErrSessionRevocationIncomplete` 并记录指标，使客户端和运维能明确感知可重试安全撤销未完成。

   备选方案：继续返回成功并只记录日志。该方案已被确认存在安全和可观测性风险。

6. 补偿采用 auth feature 内最小持久重试记录或同步失败指标优先，不引入通用 outbox。

   理由：当前仓库没有 eventbus/outbox 基础设施，本次不扩展架构边界。若实现阶段需要持久重试，应限定在 auth feature 内，承载 user-service 强制改密撤销补偿语义。

   备选方案：引入通用 outbox 或 MQ。该方案超出本次安全收紧范围，也违反当前没有真实 MQ/broker 的架构现状。

## Risks / Trade-offs

- [风险] 用户在 5 分钟内未完成改密会看到无效凭据。→ 缓解：登录可重新获取新的 password-change token；HTTP 错误保持统一无效凭据，不泄露过期或复用细节。
- [风险] Redis 不可用会阻断强制改密流程。→ 缓解：这是安全敏感路径的有意 fail-closed；登录签发和改密消费都必须依赖一次性会话创建/消费成功。
- [风险] DB 已更新但撤销投影失败时，用户收到错误但密码可能已经变更。→ 缓解：返回明确的安全撤销未完成错误，记录指标和补偿信号；再次登录可按最新状态和凭据重新进入流程。
- [风险] 新增 Redis key 和 Lua 脚本增加实现复杂度。→ 缓解：key schema、脚本结果码和错误映射集中在 auth Redis adapter，并用 miniredis 或等价 Redis 测试覆盖创建、消费、复用、过期和撤销。
- [风险] 指标标签可能引入高基数。→ 缓解：所有新增指标只允许固定 `result`、`reason`、`operation` 等枚举标签，不包含用户 ID、session ID、jti、Redis key 或原始错误。

## Migration Plan

1. 更新配置结构、示例配置和配置校验，新增 `auth.jwt.password_change_token_ttl`。
2. 增加 auth Redis password-change session adapter、key schema 和原子消费脚本。
3. 调整 token issuer TTL 选择和 login 强制改密分支，签发 token 后创建一次性 session；创建失败则不返回 token。
4. 调整 `ChangePassword`，在更新凭据前消费 session，并使用旧 `token_version` 条件更新凭据。
5. 调整撤销失败错误策略、HTTP 映射、metrics 和 alerts。
6. 重新生成 gomock、OpenAPI，并运行 auth 相关测试、架构 lint、OpenAPI 生成和仓库验证。

回滚策略：回滚本次应用代码和配置变更即可。由于不引入数据库 schema，回滚不需要 migration 回退；未消费的 password-change Redis key 会按短 TTL 自动过期。

## Open Questions

无。
