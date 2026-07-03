## ADDED Requirements

### Requirement: 仓库级 OpenAPI 转换工具

系统 MUST 将跨服务复用的 OpenAPI 转换 CLI 维护在仓库级 `tools/openapi-convert/`，并通过服务脚本传入服务专属生成参数。OpenAPI 转换核心 MUST 保持在 `common/http/openapi`，服务 `internal/` 目录 MUST NOT 承载该通用转换 CLI。

#### Scenario: user-service 生成 OpenAPI

- **WHEN** 协作者执行 `make user-service-openapi-generate`
- **THEN** user-service 生成脚本 MUST 调用 `tools/openapi-convert` 完成 Swagger 2 到 OpenAPI 3 的转换
- **AND** 系统 MUST 更新 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`

#### Scenario: 服务专属生成参数

- **WHEN** 服务生成 OpenAPI 文档时需要配置业务 server、root server、探活路径、security scheme 或输出路径
- **THEN** 对应服务脚本 MUST 显式传入这些参数
- **AND** `tools/openapi-convert` MUST NOT 写死 user-service 的 `/api/v1`、`/livez`、`/readyz`、`/startupz` 或 `BearerAuth` 作为服务语义默认值

#### Scenario: 工具归属边界

- **WHEN** 仓库维护 Swagger/OpenAPI 转换能力
- **THEN** 可复用转换库 MUST 位于 `common/http/openapi`
- **AND** 可执行转换 CLI MUST 位于 `tools/openapi-convert`
- **AND** `user-service/internal/tools/openapi-convert` MUST 不存在
