## ADDED Requirements

### Requirement: metrics primitive 文件职责与业务中立边界

`common/runtime/observability/metrics` MUST 作为跨服务共享、业务中立的 runtime primitive 维护 provider、registry、context-aware gather、runtime collector 与 adapter 职责。实现可以在同 package 内按职责拆分文件，但 MUST 保持原 package、调用方 import path、导出 API、指标名称、label、bucket 和采集语义兼容。

#### Scenario: 同 package 文件职责可独立审查

- **WHEN** metrics package 维护 Provider/registry、`ContextCollector`/gather wrapper、runtime collector 和 SQL、Redis、scheduler、workerpool、localcache、component status collector adapter
- **THEN** 这些职责 MUST 通过清晰的同 package 文件组织表达
- **AND** 文件拆分 MUST NOT 创建 `metrics` 子包或改变调用方 import path

#### Scenario: 导出 API 与指标契约保持兼容

- **WHEN** 调用方升级到整理后的 metrics package
- **THEN** 既有导出 constructor、method、interface、error 和 adapter 类型 MUST 保持可用
- **AND** 既有 Prometheus 指标名称、label name、label value 枚举、bucket 和采集语义 MUST 保持兼容
- **AND** 系统 MUST NOT 为本次重构新增业务指标或删除既有稳定指标

#### Scenario: common 不承载服务业务语义

- **WHEN** metrics package 增加文档、示例、collector 或 adapter
- **THEN** 内容 MUST 保持业务中立，MUST NOT 导入 user-service feature 包、DTO、业务状态、业务 key schema、policy loader、route diff 或服务私有配置
- **AND** 业务指标 MUST 留在所属 feature 或消费服务边界，只通过共享 provider 注册低基数 collector

#### Scenario: 测试与示例不污染生产 API

- **WHEN** 为 metrics package 增加测试或 executable examples
- **THEN** 测试 MUST 基于现有公开 API、本地 registry、局部 fixture 或内存对象验证行为
- **AND** 正式代码 MUST NOT 仅为测试便利新增全局可变函数、测试 flag、`NewXForTest` 或无运行时职责的 adapter
