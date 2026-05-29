## Why

后续变更已经在 `common/config` 中重新加入了结构化配置校验，包括 `Config.Validate()`、`common/config/validate.go` 以及 `Load` 阶段的校验调用。需要重新评估并执行 `evaluate-config-validation-policy`，将当前配置加载契约调整为“只读取与反序列化”，避免配置包在加载阶段承担 required/optional 和基础范围校验职责。

## What Changes

- 移除 `common/config.Load` 中对 `cfg.Validate()` 的调用，使配置加载只负责读取 YAML、绑定 `AEGISCORE_` 环境变量并反序列化为 `config.Config`。
- 移除当前新增的 `common/config/validate.go` 及其 required、范围、duration、连接池等校验逻辑。
- 保留命名 Redis 与 PostgreSQL 配置结构，包括 `redis.<name>`、`postgre.<name>`、`RedisConfig(name)` 和 `Postgres(name)` 查询能力。
- 保留 Viper 环境变量绑定能力，确保命名实例如 `AEGISCORE_REDIS_CACHE_REDIS_DB` 与 `AEGISCORE_POSTGRE_USER_DB_PASSWORD` 仍可覆盖。
- 调整测试：缺失字段和基础范围异常不再期望在 `Load` 阶段失败，而是验证会加载为零值或原始值；运行时依赖失败仍由 Redis/PostgreSQL/HTTP 初始化暴露。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 配置加载契约改为只读取、环境覆盖和反序列化；不在 `common/config.Load` 执行字段存在性、required/optional 或基础范围校验。

## Impact

- 影响代码：`common/config/loader.go`、`common/config/validate.go`、`common/config/loader_test.go`。
- 影响规格：`openspec/specs/shared-infrastructure/spec.md` 的配置加载要求需要从“加载时校验配置结构”调整为“加载时不校验字段，运行时初始化暴露错误”。
- 运行时行为：缺失或非法字段不会由 `Load` 返回字段级校验错误；后续启动过程可能因 Redis ping、PostgreSQL open/ping、HTTP server 创建或监听失败而终止。
- 不改变 HTTP API、响应信封、Ent schema、controller/service/repository 分层或命名 Redis/PostgreSQL 配置格式。
