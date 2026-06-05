## Context

用户服务当前通过 `config.Config` 读取 `app.name`、`app.environment` 和 `system.timezone`，并通过 Fx 注入到启动装配边界。认证会话 Redis 实现位于 `user-services/internal/repository/redis`，负责 token version 缓存、Refresh Token 会话记录和用户会话索引 key 的构造与访问。HTTP server 生命周期位于 `user-services/internal/bootstrap/server.go`，`NewHTTPServer` 在启动时已有 `starting http server` 日志并包含监听地址。

本变更复用现有配置字段，不新增配置项，不改变 `common/runtime/config.Load` 的“只读取、覆盖和反序列化”职责，也不修改路由、Swagger 注释、JWT issuer 或数据库结构。

## Goals / Non-Goals

**Goals:**

- 认证会话 Redis key 使用 `config.App.Name` 作为前缀来源。
- 不校验 `app.name`，不设置代码级默认服务名；`app.name` 为空时，key 前缀为空。
- 保持 Redis key 的业务后缀语义清晰，例如 token version、session 和用户 sessions 索引仍保留当前 `auth:*` 结构。
- `NewHTTPServer` 启动日志保留现有 `addr` 字段，并追加 `service`、`environment` 和 `timezone` 字段。

**Non-Goals:**

- 不抽取 `/api/v1`、`/auth`、`/users`、`/healthz` 等 HTTP path 常量。
- 不修改 Swagger 注释或生成方式。
- 不把 JWT issuer 强制绑定到 `app.name`。
- 不新增 Redis `key_prefix` 配置项，也不新增配置校验。
- 不兼容读取迁移前的无前缀 Redis key。

## Decisions

1. Redis key prefix 通过服务侧 Redis key builder 派生并注入 Redis repository。

   实现应在 `user-services/internal/service` 中提供 `RedisKeyBuilder`，由 `config.App.Name` 裁剪后派生认证会话 Redis key。`user-services/internal/bootstrap` 通过 Fx provider 创建该 builder，并注入 `user-services/internal/repository/redis` 的认证会话 repository。这样 key prefix 规则集中在一个可单测的服务侧组件中，Redis repository 仍负责实际 Redis 读写，service 业务编排继续依赖 `repository.AuthSessionRepository` 抽象，不接触 Redis client。

   备选方案是把 prefix 和 key 拼接作为 `authSessionRepository` 私有方法，但用户要求抽象为 builder；独立 builder 更利于后续复用到其他 Redis 数据域。

2. 空 `app.name` 不触发启动失败，也不使用 fallback。

   本变更遵循用户要求和现有配置加载原则：不校验 `app.name`，不设置默认服务名。key 构造时仅使用 `strings.TrimSpace(cfg.App.Name)`，空字符串表示无前缀。为避免生成以冒号开头的 key，空前缀时返回原业务 key，例如 `auth:session:<session_id>`。

   备选方案是强制 `app.name` 必填或 fallback 到编译期常量，但这会改变启动配置语义，不符合本变更目标。

3. 不新增 Redis key 兼容双读。

   前缀变化会使现有 Redis 会话、token version 缓存和用户 sessions 索引无法被新逻辑读取。由于这些数据都有认证会话或缓存生命周期，且 proposal 已明确兼容性影响，实现不增加旧 key 双读、迁移脚本或后台清理逻辑。

   备选方案是读新 key miss 后读旧 key，但会增加安全会话撤销语义复杂度，并延长旧 namespace 的维护周期。

4. 启动日志只追加字段，不替换现有日志。

   `NewHTTPServer` 当前启动日志包含 `addr`。实现应在同一条 `starting http server` 日志中追加 `service`、`environment`、`timezone` 字段，不删除 `addr`，不改变监听、错误返回或 `http.ErrServerClosed` 处理。

   备选方案是新增第二条日志，但会增加启动日志噪声，且不利于单条日志检索完整启动上下文。

## Risks / Trade-offs

- Redis key namespace 变化会让存量登录会话失效 -> 接受该行为；部署后用户重新登录或等待旧 TTL 过期。
- `app.name` 为空时没有前缀隔离 -> 符合“不校验、不 default”的要求；通过配置管理保证生产环境设置正确。
- `app.name` 变更会改变 Redis key namespace -> 将服务改名视为运维配置变更，部署前需评估会话失效影响。
- 启动日志字段依赖配置值，空值会直接记录为空 -> 符合不校验配置原则，便于暴露配置缺失状态。

## Migration Plan

1. 在开发环境部署后，验证新登录创建的 Redis key 包含 `app.name` 前缀。
2. 验证旧无前缀会话不再被新逻辑读取，确认登录、刷新、退出当前设备和退出全部设备行为符合预期。
3. 验证启动日志仍包含 `addr`，并追加 `service`、`environment`、`timezone`。
4. 回滚时恢复旧 key 构造逻辑；已用新前缀创建的会话会对旧逻辑不可见，用户可能需要重新登录。

## Open Questions

- 无。
