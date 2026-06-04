## MODIFIED Requirements

### Requirement: Centralize runtime resource name constants
系统 SHALL 在 `common/runtime/resources` 集中维护共享运行时资源名称常量，用于 Redis、PostgreSQL 和 Ent runtime dependency wiring。常量 MUST 组织在职责明确的资源名文件中，并通过中文注释说明其用于 datastore 和 Ent 的 Fx wiring。常量值 MUST 与现有命名实例契约保持一致。

#### Scenario: Provide user database name constant
- **WHEN** 用户服务声明或创建 `user_db` PostgreSQL pool 或 Ent client
- **THEN** 非 struct tag 的运行时资源名称引用 MUST 使用 `common/runtime/resources` 中的 `user_db` 公共常量
- **THEN** 该常量值 MUST 保持为 `user_db`

#### Scenario: Provide common database name constant
- **WHEN** 用户服务声明或创建 `common_db` PostgreSQL pool 或 Ent client
- **THEN** 非 struct tag 的运行时资源名称引用 MUST 使用 `common/runtime/resources` 中的 `common_db` 公共常量
- **THEN** 该常量值 MUST 保持为 `common_db`

#### Scenario: Provide cache redis name constant
- **WHEN** 用户服务声明或创建 `cache_redis` Redis client
- **THEN** 非 struct tag 的运行时资源名称引用 MUST 使用 `common/runtime/resources` 中的 `cache_redis` 公共常量
- **THEN** 该常量值 MUST 保持为 `cache_redis`

#### Scenario: Keep resource names in an explicit file
- **WHEN** 维护者查看共享运行时资源名常量
- **THEN** 常量 MUST 位于 `common/runtime/resources` 包中的职责明确资源名文件中
- **THEN** 常量组 MUST 使用中文注释说明其用于 datastore 和 Ent 的 Fx wiring
- **THEN** 实现 MUST NOT 为减少文件数量而将这些跨资源常量合并进配置加载、logger、datastore 或 Fx adapter 实现文件

#### Scenario: Preserve Fx name tags
- **WHEN** Go struct tag 用于 Fx named injection
- **THEN** struct tag 中的 name 值 MUST 继续匹配 `user_db`、`common_db` 或 `cache_redis`
- **THEN** 实现 MUST NOT 为替换 tag 字面量而引入改变依赖图行为的大规模 wiring 重构

#### Scenario: Preserve named datastore configuration
- **WHEN** 运行时资源名称常量迁移完成
- **THEN** 配置路径 MUST 继续使用 `postgres.user_db`、`postgres.common_db` 和 `redis.cache_redis`
- **THEN** `common/runtime/config.Load` 的读取、覆盖和反序列化行为 MUST 保持不变

### Requirement: Keep shared infrastructure constants at their contract boundary
共享基础设施 SHALL 在拥有契约的包内维护配置加载、资源名、trace-id、日志和 datastore provider 相关常量。实现 MUST 保持 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例、Fx named injection、`X-Trace-ID` header 和日志 `trace-id` 字段等外部或跨模块契约不变。

#### Scenario: Config contract constants stay with config loading
- **WHEN** 实现调整配置默认路径、环境变量前缀、key replacer 或配置结构 tag 相关常量
- **THEN** 这些常量 MUST 由 `common/runtime/config` 边界维护
- **THEN** `common/runtime/config.Load` MUST 继续只读取、覆盖和反序列化配置
- **THEN** 实现 MUST NOT 通过常量重构新增 required、optional 或基础范围校验

#### Scenario: Runtime resource names stay with resource contract
- **WHEN** 实现引用 `user_db`、`common_db` 或 `cache_redis` 运行时资源名
- **THEN** 非 struct tag 的引用 MUST 优先使用 `common/runtime/resources` 中的公共资源名常量
- **THEN** 配置路径 MUST 继续保持为 `postgres.user_db`、`postgres.common_db` 和 `redis.cache_redis`
- **THEN** Fx struct tag 中无法引用常量的 name 字面量 MUST 与公共资源名常量保持一致

#### Scenario: Trace identifier constants preserve boundary-specific names
- **WHEN** 实现整理 trace-id 相关常量
- **THEN** HTTP header MUST 保持为 `X-Trace-ID`
- **THEN** Gin 或 request context key MUST 保持当前兼容表达
- **THEN** Zap 日志字段 MUST 保持为 `trace-id`
- **THEN** 实现 MUST NOT 为追求单一名称而改变跨边界契约

#### Scenario: Logging and datastore fallback defaults remain package-owned
- **WHEN** 实现整理日志轮转默认值、Redis/PostgreSQL ping timeout 或 datastore provider fallback
- **THEN** 默认值 MUST 保持在拥有运行时行为的基础设施包附近
- **THEN** 示例 YAML 中的部署默认值与代码 fallback 不一致时 MUST 通过命名、测试或文档说明语义差异

### Requirement: Use categorized runtime paths for shared infrastructure
共享基础设施相关代码 SHALL 使用分类后的 runtime 和 HTTP adapter 路径。配置加载 MUST 位于 `common/runtime/config`；配置 Fx provider MUST 位于 `common/runtime/configfx`；Zap logger 纯逻辑 MUST 位于 `common/runtime/logger`；Zap logger Fx provider 与停止同步 lifecycle MUST 位于 `common/runtime/loggerfx`；Redis/PostgreSQL client 或连接池构造 MUST 位于 `common/runtime/datastore`；Redis/PostgreSQL 命名 Fx provider 和 ping/close lifecycle MUST 位于 `common/runtime/datastorefx`；运行时资源名 MUST 位于 `common/runtime/resources`；timezone Fx module MUST 位于 `common/runtime/timezone`；共享 HTTP middleware MUST 位于 `common/http/middleware`。目录迁移 MUST 保持配置加载、日志、trace-id、Redis/PostgreSQL provider 和 Fx lifecycle 行为不变。

#### Scenario: Runtime config path is updated without contract changes
- **WHEN** 配置加载代码位于 `common/runtime/config`
- **THEN** `Load` MUST 继续读取 YAML 并应用 `AEGISCORE_` 环境变量覆盖
- **THEN** YAML key、环境变量前缀、Redis/PostgreSQL 命名实例和不执行 required/range 校验的行为 MUST 保持不变
- **THEN** `common/runtime/config` MUST NOT import `go.uber.org/fx`

#### Scenario: Config Fx provider is separated
- **WHEN** 服务需要通过 Fx 注入共享配置
- **THEN** 配置 Fx provider MUST 位于 `common/runtime/configfx`
- **THEN** 该 provider MUST 调用 `common/runtime/config.Load` 构造 `*config.Config`

#### Scenario: Logger path is updated without behavior changes
- **WHEN** logger 纯逻辑位于 `common/runtime/logger`
- **THEN** context logger API MUST 继续输出 `trace-id` 字段
- **THEN** Zap encoder、caller 和日志文件切分语义 MUST 保持不变
- **THEN** `common/runtime/logger` MUST NOT 承担 Fx provider 职责

#### Scenario: Logger Fx lifecycle is separated
- **WHEN** 服务需要通过 Fx 注入 Zap logger
- **THEN** logger Fx provider MUST 位于 `common/runtime/loggerfx`
- **THEN** Fx app 停止时必须继续同步或关闭 logger 资源
- **THEN** stdout/stderr 等不可同步设备常见的 `syscall.EINVAL` 与 `syscall.ENOTTY` 处理语义 MUST 保持不变

#### Scenario: Datastore construction is separated from Fx lifecycle
- **WHEN** Redis client 或 PostgreSQL 连接池构造逻辑被维护
- **THEN** 纯构造逻辑 MUST 位于 `common/runtime/datastore`
- **THEN** `common/runtime/datastore` MUST NOT import `go.uber.org/fx`
- **THEN** Redis/PostgreSQL driver、DSN、连接池参数和 timeout 行为 MUST 保持不变

#### Scenario: Datastore Fx providers are explicit
- **WHEN** Redis 和 PostgreSQL provider 迁移到 `common/runtime/datastorefx`
- **THEN** provider MUST 继续只连接调用方显式声明的命名实例
- **THEN** `cache_redis`、`user_db` 和 `common_db` 的配置路径、Fx name tag 和常量值 MUST 保持不变
- **THEN** Redis 和 PostgreSQL provider MUST 继续注册启动 ping 与停止 close lifecycle

#### Scenario: HTTP middleware path is updated without request behavior changes
- **WHEN** 共享 middleware 位于 `common/http/middleware`
- **THEN** trace-id、recovery、request logging、CORS 和认证 middleware 行为 MUST 保持不变
- **THEN** `X-Trace-ID` header、Gin context trace key 和日志 `trace-id` 字段 MUST 保持不变
