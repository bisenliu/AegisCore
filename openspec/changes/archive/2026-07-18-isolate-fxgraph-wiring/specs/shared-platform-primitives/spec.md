## MODIFIED Requirements

### Requirement: Logger、观测与 Fx 装配

系统 MUST 在 `common/runtime` 中提供业务中立的 logger、metrics、tracing、Fx provider 和依赖图原语。构造函数、provider 和 Fx graph helper MUST 只消费真实运行时依赖或调用方显式提供的无副作用 Fx option，MUST NOT 为测试便利暴露生产 API 或读取服务私有配置。

#### Scenario: logger 构造无全局副作用

- **WHEN** 调用方通过 `logger.New`、`NewWithConfig` 或 Fx provider `NewLogger` 创建 logger
- **THEN** 系统 MUST 返回由调用方拥有的 logger，Fx provider MUST 注册既有 Sync 关闭 hook
- **AND** 构造过程 MUST NOT 隐式安装、覆盖或恢复进程级默认 logger
- **AND** 默认 logger 只能通过显式 `SetDefault` 修改，并 MAY 作为未注入 logger 时的兜底

#### Scenario: Fx provider 使用真实输入语义

- **WHEN** 共享 provider 的依赖类型唯一且没有 named、optional、group 或其他 DI metadata
- **THEN** provider MUST 使用普通强类型参数
- **AND** 只有存在真实 DI metadata、复杂输出映射或明显的多依赖可读性收益时 MAY 使用 Params 容器
- **AND** provider MUST 只消费跨服务配置和 primitive，不得导入服务私有配置

#### Scenario: 生成稳定依赖图

- **WHEN** 服务将 Fx option 或 module 传入 `common/runtime/fxgraph`
- **THEN** helper MUST 输出稳定排序的 provider、invoke 和依赖关系图文本
- **AND** helper MUST 只处理调用方显式传入的 graph-safe Fx option，MUST NOT 构造或要求服务私有配置、feature provider、Ent、Redis、PostgreSQL、OTLP 或 HTTP server 输入
- **AND** helper MUST NOT 通过服务完整 runtime module 间接执行生产 runtime `fx.Invoke`，也 MUST NOT 提供服务私有 fake resource、provider 替换或 graph mode 分支
- **AND** 需要规避运行时副作用时，服务侧 MUST 提供 wiring graph 或专用无副作用 graph root，而不是把服务语义下沉到 `common`

#### Scenario: 公开 API 具有运行时职责

- **WHEN** `common/runtime` 新增公开 constructor、method、option 或 hook
- **THEN** 入口 MUST 具有真实运行时职责或已定义的稳定共享契约
- **AND** 仅测试消费、暴露内部状态或绕过正常 lifecycle 的能力 MUST 留在包内、`_test.go` fixture 或 `common/testing`
