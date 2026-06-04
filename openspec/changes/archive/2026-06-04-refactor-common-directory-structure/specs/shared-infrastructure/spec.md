## ADDED Requirements

### Requirement: Use categorized runtime paths for shared infrastructure
共享基础设施相关代码 SHALL 使用分类后的 runtime 和 HTTP adapter 路径。配置加载 MUST 位于 `common/runtime/config`；Zap logger MUST 位于 `common/runtime/logger`；Redis/PostgreSQL provider、运行时资源名和 Fx helper MUST 位于 `common/runtime/infrastructure`；timezone Fx module MUST 位于 `common/runtime/timezone`；共享 HTTP middleware MUST 位于 `common/http/middleware`。目录迁移 MUST 保持配置加载、日志、trace-id、Redis/PostgreSQL provider 和 Fx lifecycle 行为不变。

#### Scenario: Runtime config path is updated without contract changes
- **WHEN** 配置加载代码迁移到 `common/runtime/config`
- **THEN** `Load` MUST 继续读取 YAML 并应用 `AEGISCORE_` 环境变量覆盖
- **THEN** YAML key、环境变量前缀、Redis/PostgreSQL 命名实例和不执行 required/range 校验的行为 MUST 保持不变

#### Scenario: Logger path is updated without behavior changes
- **WHEN** logger 代码迁移到 `common/runtime/logger`
- **THEN** context logger API MUST 继续输出 `trace-id` 字段
- **THEN** Zap encoder、caller、日志文件切分和 sync 错误处理语义 MUST 保持不变

#### Scenario: Infrastructure path is updated without wiring changes
- **WHEN** Redis 和 PostgreSQL provider 迁移到 `common/runtime/infrastructure`
- **THEN** provider MUST 继续只连接调用方显式声明的命名实例
- **THEN** `cache_redis`、`user_db` 和 `common_db` 的配置路径、Fx name tag 和常量值 MUST 保持不变

#### Scenario: HTTP middleware path is updated without request behavior changes
- **WHEN** 共享 middleware 迁移到 `common/http/middleware`
- **THEN** trace-id、recovery、request logging、CORS 和认证 middleware 行为 MUST 保持不变
- **THEN** `X-Trace-ID` header、Gin context trace key 和日志 `trace-id` 字段 MUST 保持不变
