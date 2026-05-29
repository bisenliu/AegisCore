# go-toolchain-baseline

## Purpose

Go 工具链基线能力定义 AegisCore workspace 和各 Go module 的统一语言版本、显式 toolchain 和验证方式，确保开发、测试、代码生成和自动化环境使用一致的 Go 1.26 基线。

## Requirements

### Requirement: Use Go 1.26 as the repository toolchain baseline

仓库必须将 Go 1.26 作为 workspace 和所有 Go module 的统一语言版本基线，并使用 `go1.26.3` 作为当前显式 toolchain。

#### Scenario: Workspace declares Go 1.26 baseline
- **Given** 开发者或自动化环境读取根目录 `go.work`
- **When** 检查 workspace 的 Go 版本声明
- **Then** `go.work` 必须声明 `go 1.26`
- **Then** `go.work` 必须声明 `toolchain go1.26.3`

#### Scenario: Workspace includes project modules
- **Given** 开发者或自动化环境读取根目录 `go.work`
- **When** 检查 workspace module 列表
- **Then** workspace 必须包含 `./common`
- **Then** workspace 必须包含 `./user-services`

#### Scenario: Modules declare Go 1.26 baseline
- **Given** 开发者或自动化环境读取 `common/go.mod` 和 `user-services/go.mod`
- **When** 检查每个模块的 Go 版本声明
- **Then** 每个模块必须声明 `go 1.26`
- **Then** 每个模块必须声明与 workspace 兼容的 `toolchain go1.26.3`

### Requirement: Document Go development prerequisite

开发文档必须将 Go 1.26 和当前显式 toolchain 记录为本仓库的开发、测试、Ent 生成和 Atlas helper 构建前置条件。

#### Scenario: Developer reads prerequisites
- **Given** 开发者打开 `docs/DEVELOPMENT.md`
- **When** 查看开发前置条件
- **Then** 文档必须说明 workspace 使用 `go 1.26`
- **Then** 文档必须说明显式 toolchain 使用 `go1.26.3`

### Requirement: Verify modules with Go 1.26

仓库必须在 Go 1.26 工具链下分别验证 `common` 与 `user-services` 两个模块的测试套件；仓库根目录不是单一 Go module，不应把根目录 `go test ./...` 作为唯一验证命令。

#### Scenario: Common module tests pass
- **Given** Go 1.26 工具链可用
- **When** 在 `common/` 目录运行 `go test ./...`
- **Then** 共享模块测试必须通过

#### Scenario: User services module tests pass
- **Given** Go 1.26 工具链可用
- **When** 在 `user-services/` 目录运行 `go test ./...`
- **Then** 用户服务模块测试必须通过

#### Scenario: Toolchain changes update OPSX baseline
- **Given** 开发者修改 `go.work`、`common/go.mod` 或 `user-services/go.mod` 的 `go` 或 `toolchain` 声明
- **When** 该变更准备进入主线
- **Then** 开发者必须同步更新 `openspec/specs/go-toolchain-baseline/spec.md`
- **Then** 开发者必须同步更新 `docs/DEVELOPMENT.md` 中的前置条件
