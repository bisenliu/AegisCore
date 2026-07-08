## Why

当前配置解析仍在代码中直接导入旧版 `github.com/mitchellh/mapstructure`，而依赖图已包含新版 `github.com/go-viper/mapstructure/v2`。继续混用旧版 API 会让配置 decode 行为、依赖维护和后续 Viper 生态升级存在不一致风险。

本变更统一迁移到 `mapstructure/v2`，按 v2 的标准行为调整配置解析、测试和相关说明，不为旧版行为保留兼容层。

## What Changes

- 将项目内所有 `mapstructure` 代码导入迁移为 `github.com/go-viper/mapstructure/v2`。
- 基于 v2 API 重新审视 `common/runtime/config` 的 decode hook 组合、配置反序列化调用和错误处理。
- 同步更新 Go module 依赖关系，移除旧版 `github.com/mitchellh/mapstructure` 的直接或间接残留。
- 更新覆盖配置解析的测试，验证环境变量覆盖、duration、slice 等 decode 行为符合 v2 标准。
- 同步更新相关开发文档或规格说明中涉及配置解析依赖和验证命令的内容。
- **BREAKING**：不保留旧版 `mapstructure` 行为兼容代码；如 v2 与旧版 decode 行为存在差异，直接采用 v2 标准行为并调整调用方预期。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`：配置加载 primitive 的依赖和 decode 行为统一到 `github.com/go-viper/mapstructure/v2`。

## Impact

- 影响代码：`common/runtime/config` 中的配置加载与 decode hook 组合，以及所有引用 `mapstructure` 的 Go 源码。
- 影响依赖：`common/go.mod`、`common/go.sum`、`user-service/go.mod`、`user-service/go.sum` 中的 `mapstructure` 依赖解析结果。
- 影响测试：配置加载相关单元测试需要覆盖 v2 decode 行为，普通代码验证优先运行相关 Go package 测试，最终运行 `make lint` 和 `make verify`。
- 不改变 HTTP API、数据库 schema、OpenAPI、部署资产或运行时观测端点。
