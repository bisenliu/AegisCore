## ADDED Requirements

### Requirement: Go 生成物 drift 校验

系统 MUST 将 mock 生成物和 metrics no-op 生成物纳入 Go 生成与交付验证流程。完整验证 MUST 能在生成物过期、缺失或未提交时通过 drift 检查失败。

#### Scenario: 生成 mock 和 metrics no-op
- **WHEN** 协作者执行仓库约定的 Go 生成命令
- **THEN** 系统 MUST 生成 `go.uber.org/mock/mockgen` mock 文件和 metrics no-op 文件
- **AND** 生成命令 MUST 覆盖 `common` 与 `user-service` 中声明的相关 `go:generate` 入口

#### Scenario: 完整验证发现生成物 drift
- **WHEN** mock 或 metrics no-op 源 interface 变化但生成物未同步
- **THEN** `make verify` 或等价完整验证 MUST 通过重新生成和 `git diff --exit-code` 暴露 drift
- **AND** drift 未解决前 change MUST NOT 被视为完成

#### Scenario: 工具依赖可复现
- **WHEN** 新增或更新 mock 生成入口
- **THEN** 对应 Go module MUST 显式声明 `go.uber.org/mock/mockgen` 工具依赖或等价可复现工具入口
- **AND** 生成流程 MUST NOT 依赖开发者机器上的隐式全局 `mockgen` 二进制
