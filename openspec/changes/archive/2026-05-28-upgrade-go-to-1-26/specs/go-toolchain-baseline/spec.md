## ADDED Requirements

### Requirement: Use Go 1.26 as the repository toolchain baseline
仓库 MUST 将 Go 1.26 作为 workspace 和所有 Go module 的统一语言版本基线，并使用 Go 1.26 最新可用补丁版本作为显式 toolchain。

#### Scenario: Workspace declares Go 1.26 baseline
- **Given** 开发者或自动化环境读取根目录 `go.work`
- **When** 检查 workspace 的 Go 版本声明
- **Then** `go.work` MUST 声明 `go 1.26`
- **Then** `go.work` MUST 声明 Go 1.26 最新可用补丁版本的 `toolchain`

#### Scenario: Modules declare Go 1.26 baseline
- **Given** 开发者或自动化环境读取 `common/go.mod` 和 `user-services/go.mod`
- **When** 检查每个模块的 Go 版本声明
- **Then** 每个模块 MUST 声明 `go 1.26`
- **Then** 显式 `toolchain` 声明如果存在 MUST 与 workspace 的 Go 1.26 toolchain 基线兼容

### Requirement: Document Go 1.26 development prerequisite
开发文档 MUST 将 Go 1.26 记录为本仓库的开发和测试前置条件，且不得继续引用旧 Go 1.23 或 Go 1.24 基线作为当前要求。

#### Scenario: Developer reads prerequisites
- **Given** 开发者打开 `docs/DEVELOPMENT.md`
- **When** 查看开发前置条件
- **Then** 文档 MUST 说明 workspace 使用 `go 1.26`
- **Then** 文档 MUST 说明显式 toolchain 使用 Go 1.26 最新可用补丁版本

### Requirement: Verify modules with Go 1.26
升级完成后，系统 MUST 在 Go 1.26 工具链下分别验证 `common` 与 `user-services` 两个模块的测试套件。

#### Scenario: Common module tests pass
- **Given** Go 1.26 工具链可用
- **When** 在 `common/` 目录运行 `go test ./...`
- **Then** 测试 MUST 通过

#### Scenario: User services module tests pass
- **Given** Go 1.26 工具链可用
- **When** 在 `user-services/` 目录运行 `go test ./...`
- **Then** 测试 MUST 通过
