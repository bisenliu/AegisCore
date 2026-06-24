## ADDED Requirements

### Requirement: user-service Fx 依赖图生成

系统 MUST 为 user-service 提供可执行的 Fx 依赖图生成入口，并通过带 `user-service-` 前缀的交付命令暴露给协作者。

#### Scenario: 生成 user-service 依赖图

- **WHEN** 协作者执行 user-service Fx 依赖图生成命令
- **THEN** 系统 MUST 基于 user-service 当前顶层 Fx module 生成依赖图文件
- **AND** 生成过程 MUST 复用 `common/` 中的业务中立 Fx 依赖图 helper

#### Scenario: 根 Makefile 使用服务前缀

- **WHEN** 仓库根 `Makefile` 暴露 user-service Fx 依赖图生成能力
- **THEN** 目标名称 MUST 使用 `user-service-` 前缀
- **AND** 根 `Makefile` MUST NOT 新增无服务上下文的 `fxgraph-generate`、`dependency-graph` 或等价目标

#### Scenario: 依赖图 drift 可检查

- **WHEN** user-service provider、module 或 invoke 关系变化后重新生成依赖图
- **THEN** 系统 MUST 能通过提交的生成物 diff 或专用 check 命令暴露依赖图 drift
