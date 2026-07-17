## ADDED Requirements

### Requirement: 认证资源显式生命周期

系统 MUST 将 auth session purge pool 与 token-version 本地缓存作为 auth feature 自有资源显式构造和显式关闭。auth Redis infrastructure 正式代码 MUST NOT 依赖 Fx 或 Dig 语义，MUST NOT 为关闭顺序注入仅用于 ordering 的共享 Redis client；共享 Redis client MUST 由服务资源层持有，auth 组件 MUST 只关闭自身拥有的 goroutine、队列、worker pool 和本地缓存资源。

#### Scenario: Session purge pool 普通构造
- **WHEN** user-service 构造 auth session purge pool
- **THEN** 构造器 MUST 只接收 logger、worker 配置或其他真实运行依赖
- **AND** 构造器 MUST 返回可显式停止的 pool 或接口
- **AND** 构造器 MUST NOT 接收 `fx.Lifecycle`、`dig.In` 或仅用于建立关闭顺序的 `cache_redis` 依赖

#### Scenario: Session purge pool 显式停止
- **WHEN** 服务关闭 auth session purge pool
- **THEN** pool MUST 在配置的停止超时时间内尝试 drain 已接收任务
- **AND** 重复调用停止操作 MUST 幂等，MUST NOT panic，MUST NOT 泄漏 goroutine
- **AND** 停止操作 MUST NOT 关闭共享 Redis client

#### Scenario: Token version cache 关闭契约
- **WHEN** user-service 构造 token-version 本地缓存组件
- **THEN** 构造结果 MUST 显式暴露用于校验的 cache 或 validator、用于观测的 stats，以及幂等 `Close` 操作
- **AND** enabled 模式 MUST 关闭其拥有的 localcache
- **AND** disabled 或 direct 模式 MUST 提供一致的 no-op `Close` 契约
- **AND** `Close` MUST NOT 关闭 Redis token version 投影存储或 PostgreSQL 用户存储

#### Scenario: Auth 资源先于共享 Redis 关闭
- **WHEN** user-service 通过 Fx module 装配 auth feature 与共享 Redis client
- **THEN** Fx module MUST 只在装配边界登记 auth 自有资源的关闭 hook
- **AND** auth session purge pool MUST 在共享 Redis client 关闭前完成停止
- **AND** token-version 本地缓存 MUST 在服务退出时显式关闭
- **AND** auth Redis infrastructure 正式代码中 MUST NOT 出现 `go.uber.org/fx`、`go.uber.org/dig`、`fx.Lifecycle` 或 `name:"cache_redis"` ordering-only dependency
