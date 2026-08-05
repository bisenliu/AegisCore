## Why

仓库已经把 `tools/openapi-convert` 和 `tools/nacos-config-seed` 作为正式交付链模块纳入 workspace 和根测试入口，但根 lint、GitHub Actions 的 `govulncheck` 与 `gosec` 仍只覆盖 `common` 和 `user-service`。两个工具因此没有执行同等质量门禁，现有成功输出和错误诊断写入也存在未处理错误，stdout 写失败时仍可能返回成功。

同时，`common` 与 `user-service` 的 `go.sum` 缺少 `go mod tidy` 所需的部分 `go.mod` checksum，`tools/openapi-convert` 在 `GOWORK=off` 时又无法解析仓库内 `common`。workspace 构建虽然成功，但各 module 的独立元数据维护、Renovate 和冷环境复现没有形成闭环。

## What Changes

- 为两个 `tools/*` module 增加 module-local lint 入口，并纳入根 `make lint` 与 `make verify`。
- 将 `common`、`user-service`、`tools/openapi-convert`、`tools/nacos-config-seed` 全部纳入 CI `govulncheck` 和 `gosec` matrix，使用不含路径分隔符的稳定 job/artifact 名称。
- 修复两个工具的输出写入错误处理：成功输出写失败时返回非零退出码；已经处于失败路径时，即使诊断 writer 失败也保持非零退出码。
- 为 stdout 写失败增加回归测试，并继续保持原有成功输出和错误文本契约。
- 明确 `tools/openapi-convert` 是完整仓库 checkout 内可独立维护的 module，通过相邻 `common` 的受控 `replace` 支持 `GOWORK=off go mod tidy`、test、lint 和 build；不承诺脱离仓库单独分发。
- 对全部四个 module 执行 `GOWORK=off go mod tidy`，提交稳定的 module metadata 和 checksum。
- 同步 lint 开发文档与 `delivery-operations` 规格。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`：仓库级质量和安全门禁覆盖全部四个 Go module，并明确 module metadata 与仓库内工具依赖的可维护契约。

## Impact

- 构建与质量入口：根 `Makefile`、两个工具 module 的 `Makefile`、根 `.golangci.yml`。
- CI：`.github/workflows/ci.yml` 的 `govulncheck` 和 `gosec` matrix；复用 lint workflow 继续通过 `make lint` 获得工具覆盖。
- 工具代码：`tools/openapi-convert/main.go`、`tools/nacos-config-seed/main.go` 及对应测试。
- Module metadata：`common/go.sum`、`user-service/go.sum`、`tools/openapi-convert/go.mod`、`tools/openapi-convert/go.sum`；其他 module manifest 仅在 tidy 实际需要时变化。
- 文档与规格：`docs/GO_LINT_AUTOMATION.md` 和本 OpenSpec change。
- API、数据库、OpenAPI 生成物、部署清单和观测资产不变；安全边界只扩展静态扫描覆盖范围。
