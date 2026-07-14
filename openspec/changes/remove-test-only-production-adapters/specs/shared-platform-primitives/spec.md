## ADDED Requirements

### Requirement: 共享 runtime 公开 API 不承载测试适配

`common/` 的公开 runtime constructor、method、option 和 hook MUST 具有可说明的真实运行时职责或已定义的稳定共享契约。仅被同包测试直接调用、仅用于绕过正常 lifecycle、仅用于固定测试输入或仅用于暴露内部状态的入口 MUST 留在包内实现、`_test.go` fixture 或 `common/testing`，不得作为正式导出 API 保留。

#### Scenario: 仅测试消费的构造器降为包内实现

- **WHEN** 一个 `common/runtime` 导出构造器在仓库生产调用图中没有直接消费者，主规格也未定义其独立运行时语义
- **THEN** 系统 MUST 将该构造器降为包内实现或由正式构造器内联消费
- **AND** 同包测试 MUST 通过包内 helper、Fx lifecycle fixture 或其他测试边界覆盖原行为

#### Scenario: 真实手动生命周期需求需要独立契约

- **WHEN** 新的生产调用方需要绕过 Fx lifecycle 手动创建和关闭 runtime primitive
- **THEN** 变更 MUST 明确该调用方的 owner、关闭责任、错误语义和稳定 API
- **AND** 系统 MUST NOT 仅以单元测试构造便利性为理由公开 unmanaged、for-test 或 test-hook 入口

#### Scenario: 保留真实运行时边界处理

- **WHEN** 候选代码处理协议兼容、安全失败关闭、运行时容错、观测 fallback 或已进入主规格的 helper 行为
- **THEN** 系统 MUST 以主规格和真实调用路径验证后保留该行为
- **AND** 系统 MUST NOT 因其同时方便测试或当前调用数量较少而删除真实业务边界

#### Scenario: 测试可替换性留在局部边界

- **WHEN** 测试需要注入端口失败、固定依赖返回、控制调用顺序或观察后台任务
- **THEN** 测试 MUST 优先使用消费侧最小接口、生成 mock、局部 fixture、通道或可观察状态
- **AND** 正式代码 MUST NOT 为此新增全局可变函数、nil 测试兜底、测试专用 flag 或无运行时职责的兼容 adapter
