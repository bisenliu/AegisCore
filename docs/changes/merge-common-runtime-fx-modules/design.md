# Design

## Overview

本变更是共享 runtime 包归属整理。核心思路是把 Fx 适配层视为对应 runtime 能力的一部分，而不是单独的横向 `*fx` 能力包。

目标结构：

```text
common/runtime/config/
  loader.go
  fx.go

common/runtime/logger/
  factory.go
  context.go
  writer.go
  daily_writer.go
  fx.go

common/runtime/datastore/
  redis.go
  postgres.go
  fx.go
```

如 `datastore/fx.go` 过大，可在同包内拆成 `redis_fx.go` 和 `postgres_fx.go`。无论文件如何拆分，对外包路径都应统一为 `github.com/aegiscore/common/runtime/datastore`。

## Package Moves

### Config Fx provider

将 `common/runtime/configfx/config.go` 的内容迁移到 `common/runtime/config/fx.go`：

- 包名改为 `config`。
- `ConfigPath` 类型保留。
- `NewConfig(path ConfigPath) (*Config, error)` 调用同包 `Load(string(path))`。

调用方从：

```go
fx.Supply(configfx.ConfigPath(configPath))
fx.Provide(configfx.NewConfig)
```

改为：

```go
fx.Supply(config.ConfigPath(configPath))
fx.Provide(config.NewConfig)
```

这里的命名会让 `config.NewConfig` 与现有 `config.Load` 同处一个包。`ConfigPath` 是 Fx 专用输入类型，但它仍然属于配置加载边界，放在 `config` 包内符合 owner 关系。

### Logger Fx provider

将 `common/runtime/loggerfx/logger.go` 的内容迁移到 `common/runtime/logger/fx.go`：

- 包名改为 `logger`。
- `NewLogger(lc fx.Lifecycle, cfg *config.Config) (*zap.Logger, error)` 保留现有签名。
- 内部调用同包已有 logger factory。为避免同包递归命名冲突，需要调整底层构造函数命名。

当前 `common/runtime/logger/factory.go` 已存在 `New(cfg *config.Config) (*zap.Logger, error)`。迁移后的 Fx provider 可以继续命名为 `NewLogger`，内部调用同包 `New(cfg)`：

```go
log, err := New(cfg)
```

关闭阶段仍应忽略 `syscall.EINVAL` 和 `syscall.ENOTTY`，保持 stdout/stderr 平台兼容行为。

调用方从：

```go
fx.Provide(loggerfx.NewLogger)
```

改为：

```go
fx.Provide(logger.NewLogger)
```

### Datastore Fx providers

将 `common/runtime/datastorefx/redis.go` 和 `postgres.go` 迁移到 `common/runtime/datastore`：

- 包名改为 `datastore`。
- `ProvideNamedRedis`、`NewRedisClient` 保留语义。
- `ProvideNamedPostgres`、`NewPostgres`、`NewPostgresPools` 保留语义。
- 内部调用原本同包已有底层构造函数时，需要避免名称冲突。

当前 `common/runtime/datastore/redis.go` 已存在底层 `NewRedisClient`。迁移 Fx provider 时不能在同包继续使用同名函数表达不同职责。推荐处理方式：

- 将底层 Redis 构造函数重命名为 `OpenRedisClient` 或 `NewRedisClientFromConfig`。
- 迁移后的 Fx provider 保留 `NewRedisClient(lc, cfg, log, name)`，以减少 bootstrap 调用变化。
- 更新同包内部调用和测试。

当前 `common/runtime/datastore/postgres.go` 已存在底层 `OpenPostgres(name, cfg)`，不会与 Fx provider `NewPostgres` 冲突，可保持。

迁移后的 `NewPostgresPools` 仍负责：

- 按声明 names 逐一读取 `cfg.PostgresDatabaseConfig(name)`。
- 打开连接池失败或配置缺失时关闭已打开连接池。
- 注册一个共享 Fx lifecycle hook。
- 启动阶段按声明顺序 Ping。
- 停止阶段按声明顺序 Close 并聚合错误。

迁移后的 `NewRedisClient` 仍负责：

- 读取 `cfg.RedisConfig(name)`。
- 创建 Redis client。
- Fx 启动阶段 Ping。
- Fx 停止阶段 Close。
- 使用 `logger.WithContext` 输出连接和关闭日志。

## Bootstrap Updates

`user-service/internal/bootstrap/app.go`：

- 用 `runtime/config` 替代 `runtime/configfx`。
- 用 `runtime/logger` 替代 `runtime/loggerfx`。
- `fx.Supply(config.ConfigPath(configPath))`。
- `fx.Provide(config.NewConfig, logger.NewLogger)`。

`user-service/internal/bootstrap/redis.go`：

- 用 `runtime/datastore` 替代 `runtime/datastorefx`。
- `datastore.NewRedisClient(...)` 的参数和返回语义保持不变。

`user-service/internal/bootstrap/postgres.go`：

- 用 `runtime/datastore` 替代 `runtime/datastorefx`。
- `datastore.NewPostgresPools(...)` 的参数和返回语义保持不变。

相关测试同步更新 import 和调用。

## Test Migration

`common/runtime/datastorefx/datastorefx_test.go` 覆盖的是 datastore Fx lifecycle 行为，迁移后应放在 `common/runtime/datastore` 包内。

实现时应保持测试语义：

- 缺失 PostgreSQL config 返回带 name 的错误。
- PostgreSQL pool settings 被应用。
- PostgreSQL lifecycle Ping/Close 被注册并执行。
- 多个 PostgreSQL pool 共用一个 lifecycle hook。
- Close 错误保留具名资源上下文。
- Redis 缺失配置、Ping、Close 和 named provider 行为保持不变。

如果测试当前使用 `package datastorefx` 访问未导出 helper，迁移后可以继续使用 `package datastore` 测试同包内部 helper。若要改为 `package datastore_test`，需要只通过导出 API 验证，不应为了测试扩大生产 API。

## Documentation Updates

需要同步更新长期规则文档中的旧路径：

- `AGENTS.md` Key Entry Points 中的共享配置、日志和 datastore Fx provider 路径。
- `docs/ARCHITECTURE.md` Infrastructure 中关于 logger provider 的描述。
- 其他开发或测试文档中若出现 `configfx`、`loggerfx`、`datastorefx`，应改为新 owner 包。

文档应继续保留本仓库不新增 `openspec/` 或 `docs/opsx/` 的规则。

## Compatibility

本变更不提供旧 `configfx`、`loggerfx`、`datastorefx` 包的兼容 shim。理由是当前仓库只有 `common` 和 `user-service` 两个 workspace module，迁移范围可一次性覆盖；保留 shim 会让“旧 Fx 包不再被业务代码引用”的验收变弱，也会延长结构漂移时间。

如果后续出现外部服务 module 仍依赖旧包，应在对应服务迁移 import，而不是在本仓库恢复旧 `*fx` 包。

## Verification Strategy

实现后执行：

```bash
rg -n "common/runtime/(configfx|loggerfx|datastorefx)|\\bconfigfx\\b|\\bloggerfx\\b|\\bdatastorefx\\b" common user-service AGENTS.md docs
```

该扫描不应在业务代码或长期文档中发现旧包引用。历史 change 记录如果保留旧路径，应只作为历史上下文，不应作为当前规则来源。

然后分别在模块内运行：

```bash
cd common && go test ./...
cd ../user-service && go test ./...
```

若测试失败，优先检查包内命名冲突、Fx provider import、测试包名和 Redis/PostgreSQL lifecycle 行为是否被迁移时改变。
