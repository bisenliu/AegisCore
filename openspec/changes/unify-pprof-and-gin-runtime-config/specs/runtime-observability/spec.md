## ADDED Requirements

### Requirement: pprof 与 Gin mode 使用显式运行时配置

系统 MUST 通过已解析 runtime config 控制独立 pprof 诊断 listener 和进程级 Gin mode。pprof 的启用状态和监听地址 MUST 来自 `observability.pprof`，Gin mode MUST 来自 `runtime.gin.mode`；user-service 的 Fx constructor MUST NOT 直接读取裸环境变量或在 Gin engine constructor 中隐式修改 Gin 全局 mode。

#### Scenario: pprof 未启用

- **WHEN** `observability.pprof.enabled=false`
- **THEN** user-service MUST 构造可测试的 pprof handler
- **AND** user-service MUST NOT 注册或启动 pprof listener lifecycle hook

#### Scenario: pprof 启用

- **WHEN** `observability.pprof.enabled=true` 且 `observability.pprof.addr` 合法
- **THEN** user-service MUST 使用该地址启动独立 pprof listener
- **AND** pprof listener MUST 与业务 Gin router 分离

#### Scenario: pprof 配置来源

- **WHEN** 构造 `NewPprofServer`
- **THEN** constructor MUST 只消费已解析 `*config.Config`
- **AND** constructor MUST NOT 调用 `os.LookupEnv`、`os.Getenv` 或读取 `PPROF_ENABLED`、`PPROF_ADDR`

#### Scenario: Gin mode 显式初始化

- **WHEN** user-service 正式 Fx graph 需要构造 Gin engine
- **THEN** graph MUST 先基于 `runtime.gin.mode` 显式设置 Gin mode
- **AND** `NewGinEngine` MUST NOT 调用 `gin.SetMode`
- **AND** Fx 依赖 MUST 能表达 Gin mode 初始化先于 Gin engine 构造完成
