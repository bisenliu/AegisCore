## ADDED Requirements

### Requirement: runtime primitive 内部默认值可追踪

系统 MUST 在 `common/runtime` 和 `common/testing` 中使用命名常量表达包内默认超时、轮询间隔和探测间隔，避免在核心执行路径或测试基础设施中保留难以追踪的内联时间魔法值。命名常量 MUST 保持私有，除非该值已经是明确的跨模块公开契约。

#### Scenario: scheduler 锁默认超时命名化

- **WHEN** scheduler 需要为锁释放或锁续租设置内部默认超时
- **THEN** 系统 MUST 通过 `common/runtime/scheduler` 包内私有命名常量表达该默认值
- **AND** `executor.go`、`renew.go` 和 `validation.go` MUST NOT 分别内联重复的 `5 * time.Second` 默认值

#### Scenario: 测试容器探测间隔命名化

- **WHEN** `common/testing/containers` 需要轮询 Docker mapped port 或依赖 readiness
- **THEN** 系统 MUST 通过测试 helper 包内私有命名常量表达探测或轮询间隔
- **AND** PostgreSQL 测试容器 helper MUST NOT 在端口探测循环中直接内联 `100 * time.Millisecond`

### Requirement: scheduler 任务执行流程可维护

系统 MUST 保持 `common/runtime/scheduler` 的任务执行流程职责清晰。`runJob()` MUST 作为一次任务触发的编排入口，核心子流程 MUST 通过私有函数承载执行权获取、分布式锁获取、任务上下文和续租准备、执行后 cleanup、执行结果记录，且 MUST 保持导出 API 和运行时行为不变。

#### Scenario: 拆分任务执行子流程

- **WHEN** scheduler 执行一次已注册任务
- **THEN** `runJob()` MUST 继续按本地 overlap gate、全局并发 gate、分布式锁、任务上下文、自动续租、任务执行和收尾记录的顺序编排
- **AND** 各子流程 MUST 由 `common/runtime/scheduler` 包内私有函数或私有类型承载

#### Scenario: 保持任务执行语义

- **WHEN** `runJob()` 被拆分为私有函数
- **THEN** 系统 MUST 保持任务触发、跳过原因、开始、完成、失败、panic recovery、锁释放、续租失败处理、gate 归还和 shutdown 语义不变
- **AND** 系统 MUST NOT 新增公开 executor 类型、公开接口或仅服务测试的生产适配层

### Requirement: common 模块依赖保持 tidy

系统 MUST 保持 `common` 模块依赖图与当前源码和工具入口一致。`common/go.mod` 和 `common/go.sum` MUST 通过 `GOWORK=off go mod tidy` 校验，不得手工保留当前模块不再需要的间接依赖残留。

#### Scenario: common 依赖清理

- **WHEN** `common` 模块完成 runtime primitive 或测试基础设施维护性变更
- **THEN** 系统 MUST 在 `common` 目录使用 `GOWORK=off go mod tidy` 整理依赖
- **AND** `common/go.mod` 和 `common/go.sum` MUST 只保留 Go 工具链按当前源码、测试和 tool 指令判定需要的模块项

#### Scenario: 不误删真实导入链依赖

- **WHEN** 某个间接依赖由 Gin、Swagger UI、Prometheus 或其他当前源码真实导入链带入
- **THEN** 系统 MUST 以 `go mod why -m` 和 `go mod tidy -diff` 结果为准判断是否清理
- **AND** 系统 MUST NOT 为降低依赖数量而手工删除 tidy 仍要求保留的模块项
