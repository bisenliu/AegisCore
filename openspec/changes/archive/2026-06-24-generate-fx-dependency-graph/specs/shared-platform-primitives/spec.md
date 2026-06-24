## ADDED Requirements

### Requirement: Fx 依赖图 runtime primitive

系统 MUST 在 `common/` 中提供业务中立的 Fx 依赖图构建与渲染能力，使服务可以从自身 Fx module 或 app option 生成稳定、可审查的依赖图文本。

#### Scenario: 生成业务中立依赖图

- **WHEN** 服务将 Fx option 或 module 传入共享依赖图 helper
- **THEN** 系统 MUST 返回描述 provider、invoke、输入输出依赖或等价 Fx 依赖关系的图文本
- **AND** 该 helper MUST NOT 引入 user-service feature、HTTP route、RBAC policy、Ent schema 或服务专用配置语义

#### Scenario: 输出稳定图文本

- **WHEN** 相同 Fx module 在代码未变化的情况下重复生成依赖图
- **THEN** 系统 MUST 输出稳定排序的图文本，避免产生无意义 diff

#### Scenario: 拒绝放入服务内 shared kernel

- **WHEN** Fx 依赖图能力与具体业务 feature 无关
- **THEN** 系统 MUST 将公共方法放在 `common/` 的 runtime primitive 边界
- **AND** 系统 MUST NOT 将该能力放入 `user-service/internal/shared` 或任一 feature 包
