## Why

当前共享配置结构体中 Redis 与 PostgreSQL 的字段命名不对称，降低配置代码的可读性和一致性。同时认证会话逻辑中存在多个直接写入方法体的默认 TTL 魔法值，不利于审查默认策略和后续统一调整。

## What Changes

- 将 `common/config.Config` 中表示 PostgreSQL 命名实例集合的 Go 字段从 `PostgresConfigs` 重命名为 `Postgres`，保持与 `Redis` 字段一致的单数集合命名。
- 保持外部配置契约不变：YAML key 继续为 `postgres`，`AEGISCORE_POSTGRES_*` 环境变量覆盖行为不变。
- 在认证会话相关 service/store 包内集中声明默认 Access Token TTL、Refresh Token TTL 和 token version cache TTL 等默认时间常量，并用常量替代方法体内的魔法时间值。
- 补充或调整相关测试，确保配置反序列化和认证默认 TTL 行为不因重命名或常量提取发生回归。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 标准化共享配置对象的 PostgreSQL Go 字段命名，同时保持 `postgres.<name>` 外部配置路径和运行时行为不变。
- `user-session-control`: 将认证会话默认 TTL 策略集中为包级常量，保持默认过期行为不变并提升可维护性。

## Impact

- 影响代码：`common/config/config.go`、引用 `Config.PostgresConfigs` 的 shared infrastructure provider、测试和用户服务启动装配相关代码。
- 影响代码：`user-services/internal/service/auth_service.go`、`user-services/internal/service/session_store.go` 及相关测试。
- 外部 API、HTTP 响应、数据库模型、Redis key 格式、JWT claims 和配置文件字段不应变化。
- 这是 Go 公共结构体字段命名变更，仓库内引用需要同步更新；当前不引入兼容别名，除非实现阶段发现已有外部消费者需要保留。
