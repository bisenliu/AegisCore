## MODIFIED Requirements

### Requirement: Runtime primitive 基础

系统 MUST 在 `common/runtime/` 中维护配置加载、数据存储、logger、metrics、tracing、scheduler、workerpool、localcache、Redis key 和 timezone 等 runtime primitive。`common/runtime/config` MUST 将 `local_cache` 表达为通用具名缓存实例集合，并 MUST NOT 固定 user-service 的 `auth_token_version`、`rbac_user_roles` 或其他业务缓存名。`common/runtime/config` MUST 使用 `github.com/go-viper/mapstructure/v2` 作为配置反序列化依赖，并 MUST NOT 保留旧版 `github.com/mitchellh/mapstructure` 导入、兼容层或旧行为 fallback。

#### Scenario: 服务启动加载配置

- **WHEN** 服务通过配置文件启动
- **THEN** 系统 MUST 使用共享配置 loader 与 validation 解析 runtime、HTTP、auth、Postgres、Redis、metrics、tracing、logger 和通用 `local_cache` 配置

#### Scenario: runtime 依赖初始化

- **WHEN** 服务需要连接 Postgres、Redis、logger、metrics 或 tracing provider
- **THEN** 服务 MUST 优先复用 `common/runtime/` 中的 provider 和 Fx module

#### Scenario: 后台任务执行

- **WHEN** 服务需要执行定时任务、分布式锁或固定 worker pool 任务
- **THEN** 系统 MUST 使用共享 scheduler、lock、workerpool 和 metrics 约束，并记录失败、拒绝、panic 和完成事件

#### Scenario: 本地缓存配置解析

- **WHEN** 配置文件包含 `local_cache.<name>` entry
- **THEN** `common/runtime/config` MUST 将其解析为以 `<name>` 为 key 的 `LocalCacheInstanceConfig`
- **AND** 配置 key MUST 保持原样供服务按名称读取

#### Scenario: 本地缓存配置通用校验

- **WHEN** `local_cache` 中存在一个或多个 entry
- **THEN** validation MUST 遍历所有 entry 并校验 `capacity > 0`、`ttl > 0`、`load_timeout > 0`、`num_counters >= 0` 和 `buffer_items >= 0`
- **AND** 校验错误 MUST 包含对应 `local_cache.<name>.<field>` 路径

#### Scenario: 拒绝 common 固化业务缓存名

- **WHEN** user-service 或其他服务需要声明必需本地缓存实例
- **THEN** 必需缓存名、缺失实例检查和业务含义 MUST 位于对应服务的 feature/provider 边界
- **AND** `common/runtime/config` MUST NOT 增加该业务缓存的固定字段或专用校验

#### Scenario: auth Redis key schema

- **WHEN** 认证功能需要 refresh session、token version 或撤销相关 Redis key
- **THEN** 认证 infrastructure MUST 拥有功能 key schema，只能复用 `common/runtime/rediskey` 的通用构造规则

#### Scenario: workerpool 业务边界

- **WHEN** feature 代码使用 `common/runtime/workerpool` 提交后台任务
- **THEN** workerpool MUST 只提供并发控制、生命周期、日志和统计能力，MUST NOT 承载 refresh session 上限裁剪、token version 撤销、可靠消息、eventbus、outbox 或业务一致性协议

#### Scenario: scheduler 分布式锁

- **WHEN** 定时任务具有多实例副作用
- **THEN** 任务 MUST 声明锁策略，锁 TTL MUST 为正值，长任务 SHOULD 具备续租策略

#### Scenario: mapstructure v2 配置反序列化

- **WHEN** `common/runtime/config` 将 Viper 读取到的配置反序列化为 `Config`
- **THEN** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 提供的 decode hook 和 decode 配置能力
- **AND** duration、slice、具名 Postgres、具名 Redis 和具名 `local_cache` 配置 MUST 按 v2 标准行为解析
- **AND** 系统 MUST NOT 导入 `github.com/mitchellh/mapstructure` 或保留面向旧版行为的兼容代码
