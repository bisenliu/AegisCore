## ADDED Requirements

### Requirement: 测试 mock 生成入口不参与正式构建

仓库中仅承载 `go:generate mockgen` 的 `mock_generate.go` MUST 归消费 mock 的 package 所有，并 MUST 使用 `generate` build tag 从正常 Go 构建排除。`make common-generate`、`make user-service-generate` 和完整 verify MUST 继续生成、校验并检测 mock 生成物 drift。

#### Scenario: generate-only 源文件保留本地生成入口

- **WHEN** package 通过 `mock_generate.go` 声明一个或多个 `go:generate mockgen` 指令
- **THEN** 文件 MUST 包含 `//go:build generate`
- **AND** 指令 MUST 继续输出到该消费 package 的 `_test.go` mock 生成物

#### Scenario: 正常构建排除测试生成入口

- **WHEN** 系统对 `common` 或 `user-service` package 执行普通 `go list`、`go build` 或 `go test`
- **THEN** `mock_generate.go` MUST NOT 出现在正常 package `GoFiles` 中
- **AND** 正式二进制 MUST NOT 因 mock 生成入口增加可编译生产源文件

#### Scenario: 生成命令仍可发现全部指令

- **WHEN** 开发者执行 `make common-generate` 或 `make user-service-generate`
- **THEN** Go generate MUST 使用 `generate` build tag 发现对应 `mock_generate.go`
- **AND** 所有已登记 mock MUST 能在删除或过期后被重新生成

#### Scenario: 生成物 drift 阻塞交付

- **WHEN** mock 接口、生成指令或生成物内容不一致
- **THEN** 模块 generate/verify 或仓库 `make verify` MUST 通过 `git diff --exit-code` 暴露 drift
- **AND** 系统 MUST NOT 通过跳过生成、保留旧 mock 或复制兼容文件绕过失败

### Requirement: 测试适配生产代码受交付门禁约束

仓库的架构或交付检查 MUST 拒绝明确带有测试语义且进入正式构建的 API、hook、分支或生成入口。自动检查 MUST 聚焦可确定的结构特征，其他仅测试引用的导出符号 MUST 结合主规格、生产调用图和运行时职责人工复核。

#### Scenario: 拒绝显式 test-only 正式 API

- **WHEN** 人工维护的非 `_test.go` Go 文件新增 `ForTest`、`set*ForTest`、`testHook` 或等价测试语义 symbol
- **THEN** 架构检查 MUST 失败并定位文件
- **AND** 实现 MUST 将该逻辑移入测试边界或改为具有真实运行时职责的依赖设计

#### Scenario: 拒绝未隔离的 mock 生成文件

- **WHEN** 新增或修改的 `mock_generate.go` 缺少 `generate` build tag
- **THEN** 架构或交付检查 MUST 失败
- **AND** 检查 MUST 覆盖 `common/` 与 `user-service/` 的全部人工维护 package

#### Scenario: 人工复核低调用量公开 API

- **WHEN** 扫描发现导出 symbol 只有测试消费者或注释包含测试用途
- **THEN** 实施者 MUST 检查主规格、生产调用图、跨模块职责和可替代测试手段后再决定删除或保留
- **AND** 扫描结果 MUST NOT 直接作为删除共享 API、协议兼容或安全边界的唯一依据
