## Why

当前 `user-service/internal/tools/openapi-convert/` 是 OpenAPI 生成流程使用的内嵌 CLI 工具，但它位于服务 `internal/` 目录下，容易与业务代码混放，并且在后续多服务扩展时会诱导每个服务复制一份转换工具。需要将通用转换 CLI 移到仓库级工具目录，同时保留各服务对探活路径、server、鉴权 scheme 和输出路径的独立控制。

## What Changes

- 将 OpenAPI 转换 CLI 从 `user-service/internal/tools/openapi-convert/` 迁移到仓库级 `tools/openapi-convert/`。
- 保持转换核心能力在 `common/http/openapi`，仓库级 CLI 只负责解析参数、读写文件并调用共享库。
- 调整 `user-service/scripts/openapi-generate.sh`，改为调用 `tools/openapi-convert`，并显式传入 user-service 的业务 server、root server、探活路径、BearerAuth 文案和生成来源。
- 去除仓库级 CLI 中对 user-service 探活路径、`/api/v1` 和鉴权名称的服务语义默认值，避免影响未来服务复用。
- 保持 `make user-service-openapi-generate`、`make verify` 和 OpenAPI 生成物路径不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 明确跨服务 OpenAPI 转换 CLI 必须位于仓库级 `tools/openapi-convert/`，服务脚本负责传入服务专属生成参数，服务 `internal/` 目录不得承载该通用转换 CLI。

## Impact

- 影响 `user-service/internal/tools/openapi-convert/`、`tools/openapi-convert/`、`go.work` 和 `user-service/scripts/openapi-generate.sh`。
- 影响 OpenAPI 生成流程，但不改变 `make user-service-openapi-generate` 对外命令、生成物位置或 HTTP API 行为。
- 不涉及数据库 schema、migration、运行时部署、外部 API 响应契约或 RBAC 授权语义变化。
- 需要通过 `make user-service-openapi-generate` 验证生成物无非预期 drift，并通过 `make user-service-architecture-lint` 验证目录边界。
