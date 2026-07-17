## MODIFIED Requirements

### Requirement: 架构边界与 Fx 依赖图验证

系统 MUST 通过 `user-service-architecture-lint` 保护 feature-first、分层、共享边界、生成配置和部署契约，并提供无外部副作用、无运行时激活副作用的正式 Fx 依赖图诊断入口。

#### Scenario: feature 与共享边界

- **WHEN** 服务业务代码新增到横向 controller/service/repository 或错误归入 `common`、`internal/shared`、`internal/integration`
- **THEN** 架构 lint MUST 失败并要求代码回到所属 feature 或符合既有共享准入规则
- **AND** 当前不存在的 gRPC、MQ、eventbus 或 outbox 模型 MUST NOT 以空壳或推测性实现进入正式边界

#### Scenario: 分层依赖保护

- **WHEN** domain、application 或 infrastructure 新增 import
- **THEN** lint MUST 阻止 domain 导入 Gin、Ent、Redis、config、logger、response、application port 或 adapter
- **AND** application MUST NOT 导入 HTTP transport DTO 或 Ent predicate 包
- **AND** HTTP controller MUST 先使用 `binding.BindOrAbort`，再使用 feature-local input preparer

#### Scenario: 生成 Fx 图

- **WHEN** 执行 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot`
- **THEN** 系统 MUST 基于正式配置投影和无运行时激活的 wiring graph 或专用 graph root 生成非空 DOT
- **AND** 图 MUST 展示 auth、user、role、permission、providers、router 及关键 metrics 依赖边
- **AND** 生成过程 MUST NOT 执行生产 runtime `fx.Invoke`，MUST NOT 连接真实 PostgreSQL、Redis、OTLP 或启动 listener
- **AND** 生成过程 MUST NOT 创建 workerpool、本地缓存、tracing exporter 后台资源，MUST NOT 注册真实 route 或 runtime metrics，MUST NOT 修改 `TZ`、`time.Local` 或 Gin mode

#### Scenario: 正式 App 保持完整运行时激活

- **WHEN** user-service 通过 `serve` 命令构建正式 Fx App
- **THEN** 系统 MUST 使用同时包含 wiring module 和 runtime module 的正式 App module
- **AND** HTTP server、pprof server、route 注册、runtime dependency metrics、timezone 初始化、RBAC lifecycle 和 lifecycle hooks MUST 保持正式运行时语义
- **AND** graph 命令的无副作用 root MUST NOT 取代正式 `serve` 的 runtime 激活链路
