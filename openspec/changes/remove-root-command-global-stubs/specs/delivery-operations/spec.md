## ADDED Requirements

### Requirement: user-service 根命令测试局部依赖注入

`delivery-operations` 的 user-service 根命令构造 MUST 通过命令实例本地依赖注入支持测试替身，正式代码 MUST NOT 为 `serve` lifecycle app factory 或 RBAC runner 暴露 package-level 可变函数变量。该约束 MUST 保持现有 CLI command graph、flag 默认值、配置路径传递和 `serve` lifecycle shutdown 语义不变。

#### Scenario: serve 生命周期测试使用局部 factory

- **WHEN** `user-service/cmd` 测试覆盖 `serve` 启动和停止 lifecycle
- **THEN** 测试 MUST 在当前命令或函数调用范围内传入 lifecycle app factory 替身
- **AND** 测试 MUST NOT 通过赋值 package-level `newLifecycleApp` 或等价可变全局函数变量注入替身

#### Scenario: 根命令 surface 测试不共享全局替身

- **WHEN** `user-service/cmd` 测试构造 root command 并检查 `serve`、`rbac` 或 `fxgraph` command surface
- **THEN** 每个测试 MUST 使用本地构造的命令依赖或默认依赖
- **AND** 测试 MUST NOT 依赖保存、覆盖和恢复 package-level runner 变量来避免执行真实命令

#### Scenario: CLI 外部行为保持不变

- **WHEN** 运维执行 `aegiscore-user-services serve` 或查看 root command 帮助
- **THEN** 系统 MUST 保持现有 command 名称、flag 名称、flag 默认值、配置路径默认值、输出语义和退出码
- **AND** 本次依赖注入重构 MUST NOT 修改 Makefile 目标、OpenAPI 生成物、Ent schema、Atlas migration 或部署资产
