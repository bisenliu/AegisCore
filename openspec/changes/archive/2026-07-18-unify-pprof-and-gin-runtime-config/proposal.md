## Why

当前 pprof 诊断监听由 `NewPprofServer` 在 Fx constructor 中直接读取 `PPROF_ENABLED` 和 `PPROF_ADDR`，绕过已解析配置对象、统一环境变量覆盖和配置校验路径。`NewGinEngine` 每次构造 Gin engine 时调用 `gin.SetMode(gin.ReleaseMode)`，把进程级全局状态隐藏在普通 provider constructor 中，导致多 App、并行测试和 graph 工具构造路径的行为不显式。

## What Changes

- **BREAKING**：移除裸环境变量 `PPROF_ENABLED` 和 `PPROF_ADDR` 的运行时读取，不保留兼容路径。
- **BREAKING**：pprof 配置改由统一配置系统提供，使用 `observability.pprof.enabled` 和 `observability.pprof.addr`，环境覆盖使用 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED` 和 `AEGISCORE_OBSERVABILITY_PPROF_ADDR`。
- 新增进程级 Gin mode 配置 `runtime.gin.mode`，环境覆盖使用 `AEGISCORE_RUNTIME_GIN_MODE`。
- pprof 地址格式、生产类环境 loopback 约束和 Gin mode 取值必须在配置加载校验阶段完成。
- `NewPprofServer` 只消费已解析配置对象，不再读取进程环境。
- `NewGinEngine` 不再设置 Gin 全局 mode；Gin mode 由 bootstrap 显式配置并通过 Fx graph 表达依赖顺序。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`：独立 pprof 诊断监听的启用状态、监听地址和安全约束改为统一配置系统管理；Gin mode 作为运行时启动配置显式应用。
- `shared-platform-primitives`：共享 runtime 配置契约扩展 pprof 与 Gin mode 字段、默认值、环境变量覆盖和校验规则。

## Impact

- 影响 `common/runtime/config/` 的配置结构、默认值、环境覆盖 key 和校验逻辑。
- 影响 `user-service/configs/config.yaml` 的运行时与可观测性配置示例。
- 影响 `user-service/internal/bootstrap/pprof.go` 和相关 pprof 测试。
- 影响 `user-service/internal/providers/gin.go`、`user-service/internal/providers/fx.go` 和相关 Gin engine 测试。
- 不改变 HTTP 业务 API、OpenAPI 契约、数据库 schema、Ent 生成物、Atlas migration 或部署资源规格。
- 运维侧需要把旧 `PPROF_ENABLED`、`PPROF_ADDR` 替换为 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED`、`AEGISCORE_OBSERVABILITY_PPROF_ADDR`；如需非默认 Gin mode，使用 `AEGISCORE_RUNTIME_GIN_MODE`。
