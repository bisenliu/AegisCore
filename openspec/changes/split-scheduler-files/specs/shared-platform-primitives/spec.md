## ADDED Requirements

### Requirement: scheduler 包内结构保持稳定契约

系统 MUST 允许 `common/runtime/scheduler` 按公开配置类型、调度器生命周期、任务执行流程、分布式锁、锁续租和校验逻辑拆分包内文件，同时保持 `package scheduler`、导出 API、错误变量、metrics 事件、日志语义、cron parser、并发控制、锁策略、续租策略和 shutdown 行为不变。

#### Scenario: 拆分 scheduler 源码文件

- **WHEN** `common/runtime/scheduler` 将 `scheduler.go` 中的类型、生命周期、执行、锁续租或校验逻辑移动到不同源码文件
- **THEN** 系统 MUST 保持原有导出符号、函数签名、常量值、错误语义和调用方导入路径不变
- **AND** scheduler 的任务注册、触发、跳过、失败、panic recovery、完成、分布式锁获取、自动续租和优雅关闭行为 MUST 不变

#### Scenario: 保持业务中立边界

- **WHEN** scheduler 包内结构被拆分或命名调整
- **THEN** `common/runtime/scheduler` MUST 继续只承载无业务语义 runtime primitive
- **AND** 系统 MUST NOT 将 user-service feature 语义、业务 Redis key schema、HTTP controller、Fx provider、Ent、部署资产或观测 dashboard 逻辑放入 scheduler 包
