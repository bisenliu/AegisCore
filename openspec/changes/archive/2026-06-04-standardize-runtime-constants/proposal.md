## Why

当前项目中的运行时超时、默认值、资源名、业务阈值和配置契约常量分布在 CLI、bootstrap、common 基础设施、认证服务、DTO、Ent schema、路由和脚本中。`user-services/cmd/main.go` 的 `stopTimeout = 15s` 与 `user-services/internal/bootstrap/server.go` 的 `defaultShutdownTimeout = 10s` 以及示例配置 `http.shutdown_timeout = 25s` 同时存在，容易让维护者误判“进程停止预算”和“HTTP graceful shutdown 预算”的关系，并可能导致配置的 HTTP 关闭时长被外层 Fx stop context 提前截断。

## What Changes

- 审查并标准化运行时关闭超时命名，明确 CLI/Fx start-stop timeout 与 HTTP server shutdown timeout 的职责边界。
- 调整默认 shutdown timeout 的组织方式，使配置值、代码 fallback 和外层停止预算之间的关系可读、可测试且不互相遮蔽。
- 建立常量分层原则：跨模块契约常量集中在对应公共能力包内，能力内业务规则就近放在 domain/DTO/schema 等归属包内，测试数据和示例值不做全局集中。
- 识别并收敛重复或语义冲突的默认值，包括认证 token TTL、session/cache TTL、服务名、资源名、DTO 与 Ent schema 的长度限制、路由/Swagger/API path、响应码与消息、配置路径和 migration 路径。
- 增加必要测试或规格约束，验证关闭超时不会因命名或预算不一致产生行为混淆。
- 不引入“一刀切”的全局 constants 包，不改变现有 HTTP API、YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例、响应码或数据库 schema 契约。

## Capabilities

### New Capabilities
- `runtime-constant-governance`: 定义项目内常量、默认值和配置契约的组织原则，覆盖哪些常量应集中、哪些应保持就近定义，以及重复默认值的处理规则。

### Modified Capabilities
- `http-service-runtime`: 明确 CLI/Fx 生命周期 timeout 与 HTTP graceful shutdown timeout 的命名、取值和嵌套预算关系，避免配置 shutdown timeout 被外层 stop timeout 隐式截断。
- `shared-infrastructure`: 明确配置默认值、命名运行时资源、环境变量前缀、trace-id/logging 等跨模块契约常量的归属边界。
- `common-credentials`: 明确认证传输常量、JWT subject/claim 常量、token TTL fallback 和密码参数等凭证常量的归属与复用边界。

## Impact

- 主要影响 `user-services/cmd/main.go`、`user-services/internal/bootstrap/server.go`、`user-services/configs/config.yaml`、`common/config/`、`common/infrastructure/`、`common/auth/`、`common/password/`、`common/response/`、`common/middleware/`、`user-services/internal/service/`、`user-services/internal/dto/`、`user-services/internal/domain/`、`user-services/internal/router/` 和 `user-services/ent/schema/`。
- 外部 HTTP 路由、响应信封、错误码、YAML 配置 key、环境变量覆盖格式、Redis/PostgreSQL 命名实例和数据库字段保持兼容。
- 可能调整代码中的常量文件布局、常量命名、示例配置值或外层 stop timeout 默认值；如外层停止预算被加大，行为上只会允许已配置的 HTTP graceful shutdown 更完整执行。
