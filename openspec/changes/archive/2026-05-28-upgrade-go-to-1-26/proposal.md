## Why

当前仓库的 Go 版本基线不一致：`go.work` 使用 `go 1.24` 与 `toolchain go1.24.1`，`user-services/go.mod` 使用 `go 1.24`，而 `common/go.mod` 仍为 `go 1.23`。升级到 Go 1.26 最新版可以统一 workspace 与模块工具链，降低本地开发、依赖解析和测试环境之间的版本偏差。

## What Changes

- 将仓库 Go workspace 和所有 Go module 的 `go` 版本统一升级到 `1.26`。
- 将显式 `toolchain` 指令升级到 Go 1.26 最新补丁版本，并保持 workspace 与模块声明一致。
- 更新开发文档中的 Go 前置条件，避免继续引用 Go 1.24 或旧 toolchain。
- 验证 `common` 与 `user-services` 两个模块在 Go 1.26 下可以完成测试。
- 不变更 HTTP API、响应信封、配置 key、数据库 schema 或运行时业务行为。

## Capabilities

### New Capabilities
- `go-toolchain-baseline`: 定义仓库 Go workspace 与模块必须使用的 Go 1.26 工具链基线，以及升级后的验证要求。

### Modified Capabilities
- 无。`user-profile-query`、`http-service-runtime`、`shared-infrastructure` 和 `api-response-contract` 的外部可观察需求不变。

## Impact

- 影响文件预计包括 `go.work`、`common/go.mod`、`user-services/go.mod` 和 `docs/DEVELOPMENT.md`。
- 可能影响本地开发和 CI 环境的 Go 安装版本要求；运行测试的环境需要可用的 Go 1.26 最新补丁版本。
- 不涉及 API 兼容性、错误码、配置格式、数据库模型或 Ent 生成代码变更。
