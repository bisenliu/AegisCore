## Context

user-service 当前已经采用“CLI 先解析 service config，composition root 再 supply 同源 service config 和 runtime config”的启动模型，但仍有两个运行时配置点绕过该模型：`NewPprofServer` 在 Fx constructor 中直接读取 `PPROF_ENABLED` 和 `PPROF_ADDR`，`NewGinEngine` 在普通 engine constructor 中调用 `gin.SetMode(gin.ReleaseMode)` 修改 Gin 包级全局状态。

该变更跨 `common/runtime/config` 和 `user-service/internal/bootstrap|providers`。`common` 负责声明业务中立的配置结构、默认值和校验；`user-service` 负责按已解析 runtime config 组合 pprof lifecycle、Gin mode 初始化和 HTTP engine。不得把 user-service 业务配置、feature 语义或临时测试适配放入 `common`。

## Goals / Non-Goals

**Goals:**

- 将 pprof 的启用状态和监听地址纳入统一配置对象，统一使用 `AEGISCORE_` 前缀环境变量覆盖。
- 在配置加载阶段校验 pprof 地址格式、生产类环境 loopback 约束和 Gin mode 取值。
- 让 `NewPprofServer` 只消费已解析配置，不再读取进程全局环境。
- 让 `NewGinEngine` 不再修改 Gin 全局 mode。
- 通过显式 bootstrap provider 和 Fx 依赖 marker 表达 Gin mode 必须先于 Gin engine 构造完成。
- 更新配置示例、OpenSpec delta 和相关测试，使测试不再依赖裸 pprof 环境变量。

**Non-Goals:**

- 不改变 HTTP 业务 API、认证、RBAC、OpenAPI 文档内容或数据库 schema。
- 不新增部署清单、Prometheus rule、Grafana dashboard 或 pprof 鉴权机制。
- 不保留 `PPROF_ENABLED`、`PPROF_ADDR` 旧环境变量兼容读取。
- 不为单元测试引入生产无关的多余接口、fallback 或双路径实现。

## Decisions

1. pprof 配置放入 `observability.pprof`。

   pprof 属于运行时诊断与观测边界，和 metrics/tracing 同处 `observability` 更符合现有能力地图。备选方案是放入 `server.pprof`，但 pprof 不属于业务 HTTP/gRPC protocol server，且其访问边界、安全校验和运维语义更接近诊断能力，因此不采用。

2. Gin mode 配置放入 `runtime.gin.mode`。

   Gin mode 是进程级 runtime 行为，不属于单个 HTTP server 地址、端口或超时配置，也不属于业务 feature。备选方案是放入 `server.http.gin_mode`，但该配置会修改 Gin 包级全局状态，并非单个 HTTP server 实例属性，因此不采用。

3. 所有新配置字段由 `common/runtime/config` 声明默认值和校验。

   `common/runtime/config` 已拥有跨服务核心 app、runtime、server、log、observability 配置，pprof 和 Gin mode 都是业务中立配置。`user-service/internal/config` 只应继续声明服务私有认证、RBAC cache、Ent 和具名资源配置。

4. `NewPprofServer` 直接读取 `params.Config.Observability.Pprof`。

   constructor 不再调用 `os.LookupEnv`，也不再保留 `loadPprofSettings(lookup)` 这类环境读取 helper。配置解析、默认值、环境覆盖和校验全部提前完成，Fx graph 可以通过 `*config.Config` 表达真实依赖。

5. Gin mode 使用显式 provider 加 Fx marker 保证顺序。

   新增类似 `GinModeConfigured` 的零值 marker，由 bootstrap/provider 在应用已解析配置后调用 `gin.SetMode(cfg.Runtime.Gin.Mode)` 并返回。`GinParams` 显式依赖该 marker，保证 `NewGinEngine` 被执行前 Gin mode 已配置。备选方案是只在 `RuntimeModule` 中 `fx.Invoke(ConfigureGinMode)`，但 Fx 不保证普通 constructor 和 invoke 的书写顺序足以表达该依赖，因此不采用。

6. 不保留兼容方案。

   旧裸环境变量读取会继续制造双配置源和不一致校验路径。本 change 直接删除旧路径，运维侧必须迁移到 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED`、`AEGISCORE_OBSERVABILITY_PPROF_ADDR` 和 `AEGISCORE_RUNTIME_GIN_MODE`。

## Risks / Trade-offs

- 旧环境变量不再生效 → 通过 proposal、tasks 和配置示例明确迁移到 `AEGISCORE_` 前缀变量。
- Gin mode 仍是 Gin 包级全局状态 → 通过显式启动配置和 Fx marker 把副作用集中在 composition root，不在普通 engine constructor 中隐式发生；测试需要自行隔离或恢复 Gin mode。
- `common/runtime/config` 增加 Gin 字符串配置但不应引入 Gin 依赖 → 使用稳定字符串常量和值校验，避免 `common` 依赖 Gin 包。
- pprof 地址校验从 constructor 前移到配置加载 → 启动失败更早暴露；需要同步更新配置测试和 pprof 测试断言。
- 配置字段扩展会影响未知字段校验 → 必须同步 `setCoreDefaults`、配置结构、校验和 `user-service/configs/config.yaml`，避免新字段被视为未知。

## Migration Plan

1. 扩展 `common/runtime/config` 的结构、默认值和校验。
2. 更新 `user-service/configs/config.yaml`，新增 `runtime.gin.mode` 与 `observability.pprof` 示例。
3. 调整 `NewPprofServer` 消费 `params.Config.Observability.Pprof`，删除直接环境读取路径。
4. 新增显式 Gin mode 配置 provider 和 marker，让 `NewGinEngine` 依赖 marker 并删除 `gin.SetMode(gin.ReleaseMode)`。
5. 更新测试，使用配置对象和统一 loader 环境变量覆盖验证新行为。
6. 运行相关包测试、架构 lint 和完整验证。

回滚策略：如果应用启动失败，可恢复上一版本代码和旧配置；由于不涉及数据库 schema、OpenAPI 或持久化数据，无数据回滚步骤。配置层面回滚时需要同步恢复旧裸 `PPROF_*` 环境变量路径对应代码，否则旧变量不会生效。

## Open Questions

无。
