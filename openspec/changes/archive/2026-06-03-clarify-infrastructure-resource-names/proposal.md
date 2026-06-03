## Why

`common/infrastructure/names.go` 只包含共享运行时资源名常量，文件名较泛，维护者不容易从文件名判断其职责。将文件名调整为更明确的 `resource_names.go` 并补充中文注释，可以保留集中常量的优势，同时减少阅读时的歧义。

## What Changes

- 将 `common/infrastructure/names.go` 重命名为 `common/infrastructure/resource_names.go`。
- 为 `NameUserDB`、`NameCommonDB`、`NameCacheRedis` 常量组补充中文注释，说明其用于 datastore 和 Ent 的 Fx wiring。
- 保持常量名和值不变，继续作为 `shared-infrastructure` 的集中运行时资源名契约。
- 不将常量合并进 `config.go`，避免把跨 Redis/PostgreSQL/Ent wiring 的公共命名约定误表达为配置加载实现细节。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 明确共享运行时资源名常量应集中维护在职责清晰的资源名文件中，并通过注释说明其 wiring 用途；常量值和外部配置契约保持不变。

## Impact

- 受影响代码：`common/infrastructure/names.go` 将重命名为 `common/infrastructure/resource_names.go`。
- API 兼容性：不改变 Go 常量名、包名、HTTP API、响应信封或错误码。
- 配置兼容性：不改变 YAML key、`AEGISCORE_` 环境变量、`postgres.user_db`、`postgres.common_db` 或 `redis.cache_redis`。
- 数据模型与迁移：不修改 Ent schema、生成代码、Atlas migration 或数据库结构。
