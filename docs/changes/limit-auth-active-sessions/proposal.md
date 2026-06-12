# Limit auth active sessions

## What

为认证会话增加“每用户最大活跃 session 数”治理策略。用户仍允许多端登录，但同一用户的 Redis refresh session 不再无限累积；当新登录或 refresh rotation 产生新 session 后，系统同步裁剪最旧的超限 session。

默认策略建议：

```text
auth.max_active_sessions_per_user: 5
auth.refresh_token_rotation: true
auth.jwt.refresh_token_ttl: 168h
exceed_policy: revoke_oldest
```

核心行为：

- 登录成功会创建新的 refresh session。
- 同一用户活跃 refresh session 超过上限时，删除最旧 session key 并从用户 session sorted-set 移除。
- refresh token rotation 继续同步消费旧 session 并创建新 session，同时执行上限裁剪，让历史超量 session 在刷新路径自然收敛。
- 退出当前设备仍同步删除当前 session。
- 退出全部设备、强制改密、管理员封禁等批量撤销场景继续采用“同步失效、异步物理清理”的模式。

## Why

当前认证实现支持多会话模型：每次登录都会生成新的 `session_id` 并写入 Redis，refresh session TTL 来自 `auth.jwt.refresh_token_ttl`，默认是 `168h`。这能支持多设备登录，但缺少企业级 session 上限治理。

如果用户、测试脚本或客户端反复登录，Redis 会在 7 天窗口内保留大量 `auth:session:{user_id}:<session_id>` key。虽然这些 key 会随 TTL 自动过期，也可以通过退出全部设备清理，但从安全和运营角度看仍有问题：

- 同一账号可长期保留过多活跃 refresh session，泄露面扩大。
- Redis key 数量随登录次数线性增长，缺少明确上限。
- 用户无法通过系统策略自动淘汰旧设备。
- refresh token rotation 只能处理同一 token 链路上的旧 session，不能治理其他登录产生的 session。

企业项目通常采用受控多会话：允许多个设备并存，但配置最大活跃数，超过上限时淘汰最旧 session。

## Scope

包括：

- 新增 `auth.max_active_sessions_per_user` 配置，默认值为 `5`。
- 更新 config struct、YAML 示例、环境变量覆盖和配置校验。
- 在 auth application 层持有 session 上限策略，并通过 application-owned port 传递给 Redis adapter。
- 调整 auth session application port 和 Redis adapter，使 `CreateSession` 和 `RotateSession` 支持同步上限裁剪。
- 用 Redis Lua 脚本原子完成 session 写入、索引更新、过期索引清理和超限 session 删除。
- refresh rotation 成功路径也执行上限裁剪，避免历史超量 session 长期保留。
- 保持 `logout-all` 的现有 workerpool 异步物理清理模型，不把 session 上限裁剪放入 workerpool。
- 补充 Redis adapter、auth command 和配置测试。
- 更新 `docs/ARCHITECTURE.md` 和 `AGENTS.md`，说明 auth session 上限治理和同步/异步边界。

不包括：

- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不新增 `/opsx:apply` 流程。
- 不新增新的 HTTP API、设备列表 API、设备命名或用户端设备管理 UI。
- 不新增 MQ、eventbus、outbox、dispatcher、Redis Stream、Kafka、RabbitMQ 或 NATS。
- 不修改 Ent schema、PostgreSQL migration 或用户表结构。
- 不修改 JWT claim schema、token version 语义或 refresh token TTL 默认值。
- 不把 auth session 业务策略放入 `common/runtime/workerpool`。
- 不让 auth application 直接依赖 Redis client、Lua 脚本或 workerpool。

## Acceptance Criteria

- 默认配置下，同一用户连续登录 6 次后，Redis 中最多保留 5 个活跃 refresh session。
- 被淘汰的最旧 session key 不存在，且用户 session sorted-set 中不再包含对应 session ID。
- `auth.max_active_sessions_per_user: 0` 可显式保留旧行为，不限制活跃 session 数。
- 负数 session 上限配置启动校验失败。
- 并发登录同一用户后，最终活跃 session 数不超过配置上限。
- refresh rotation 成功后，旧 session 不存在，新 session 存在，且用户总 session 数不超过配置上限。
- session 上限裁剪失败时登录或刷新返回错误，不留下“已签发 token 但 Redis 策略未落地”的成功响应。
- `logout-all` 仍同步递增 token version，使旧 token 立即失效；旧 Redis session key 的批量物理删除继续通过 `auth_session_purge_pool` 异步执行。
- `common/runtime/workerpool` 不承载 max session 业务规则，也不被 auth application 层直接依赖。
- `make test-common` 和 `make test-user-service` 通过，或明确说明未运行原因。
