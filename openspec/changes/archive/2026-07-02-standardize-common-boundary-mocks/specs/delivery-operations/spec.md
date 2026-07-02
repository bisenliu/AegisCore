## ADDED Requirements

### Requirement: common mockgen 交付验证

系统 MUST 为 `common` 模块提供可复现的 mockgen 工具入口、生成命令和 drift 校验。仓库完整验证 MUST 覆盖 `common` 中声明的 mock 生成物，生成物过期、缺失或未提交时 MUST 通过 `git diff --exit-code` 或等价 drift 检查失败。

#### Scenario: common 模块声明 mockgen 工具依赖

- **WHEN** `common` 新增或更新 mock 生成入口
- **THEN** `common` Go module MUST 显式声明 `go.uber.org/mock/mockgen` 工具依赖或等价可复现工具入口
- **AND** 生成流程 MUST NOT 依赖开发者机器上的隐式全局 `mockgen` 二进制

#### Scenario: common 生成命令覆盖 go generate

- **WHEN** 协作者执行 `make -C common generate` 或根 `make common-generate`
- **THEN** 系统 MUST 执行 `common` 模块内的 `go generate ./...`
- **AND** 该命令 MUST 覆盖 `common/security/casbin` 和 `common/http/middleware` 中声明的 mockgen 入口

#### Scenario: common verify 暴露生成物 drift

- **WHEN** 协作者执行 `make common-verify` 或 `make -C common verify`
- **THEN** 系统 MUST 运行 common lint、common 生成和 common 测试
- **AND** 系统 MUST 通过 drift 检查暴露 common 生成物缺失、过期或未提交

#### Scenario: 完整 verify 覆盖 common 生成物

- **WHEN** 协作者执行根 `make verify`
- **THEN** 系统 MUST 在完整验证链路中执行 common 生成命令
- **AND** 最终 `git diff --exit-code` MUST 能暴露 common mock 生成物 drift 或未纳入暂存区的意外变更
