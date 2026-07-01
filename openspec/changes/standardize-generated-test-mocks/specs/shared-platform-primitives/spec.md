## ADDED Requirements

### Requirement: 测试 mock 生成规范

系统 MUST 为高重复 interface 测试 double 提供生成化 mock 规范，并保持 mock 生成物归属于接口消费侧 feature-local 测试包。系统 MUST NOT 创建全局 `mocks/`、`testmocks/`、`common/mocks/` 或等价中央 mock 仓库来承载跨 feature mock。

#### Scenario: 生成 feature-local mock
- **WHEN** application、transport 或 infrastructure 测试需要替代高重复 store、notifier、policy engine、session store 或 metrics recorder interface
- **THEN** 测试 MUST 优先使用 `go.uber.org/mock/mockgen` 生成的 feature-local mock
- **AND** mock 生成物 MUST 放在接口消费侧 package 或其测试专用子包内
- **AND** mock 生成物 MUST NOT 放入中央 mock 仓库

#### Scenario: 保留状态型测试 harness
- **WHEN** 测试需要复杂内存状态、E2E 流程状态或比 expectation mock 更清晰的领域测试夹具
- **THEN** 测试 MAY 保留 package-local 测试 harness
- **AND** 该对象 MUST 使用 `testHarness`、`testStore`、`recordingMetrics` 或等价描述性名称
- **AND** 该对象 MUST NOT 作为跨 feature 共享 mock 导出

#### Scenario: 禁止测试驱动生产冗余接口
- **WHEN** 为了生成 mock 或迁移测试 double 调整代码
- **THEN** 系统 MUST NOT 仅为单元测试新增与业务无关的生产接口、分支、适配层或 `NewXForTest` 入口
- **AND** 测试 MUST 基于现有 feature/application 边界和合理的可测试性设计
