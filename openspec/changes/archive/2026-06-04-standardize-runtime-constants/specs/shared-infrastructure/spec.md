## ADDED Requirements

### Requirement: Keep shared infrastructure constants at their contract boundary
共享基础设施 SHALL 在拥有契约的包内维护配置加载、资源名、trace-id、日志和 datastore provider 相关常量。实现 MUST 保持 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例、Fx named injection、`X-Trace-ID` header 和日志 `trace-id` 字段等外部或跨模块契约不变。

#### Scenario: Config contract constants stay with config loading
- **WHEN** 实现调整配置默认路径、环境变量前缀、key replacer 或配置结构 tag 相关常量
- **THEN** 这些常量 MUST 由 `common/config` 边界维护
- **THEN** `common/config.Load` MUST 继续只读取、覆盖和反序列化配置
- **THEN** 实现 MUST NOT 通过常量重构新增 required、optional 或基础范围校验

#### Scenario: Runtime resource names stay with infrastructure wiring
- **WHEN** 实现引用 `user_db`、`common_db` 或 `cache_redis` 运行时资源名
- **THEN** 非 struct tag 的引用 MUST 优先使用 `common/infrastructure` 中的公共资源名常量
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
