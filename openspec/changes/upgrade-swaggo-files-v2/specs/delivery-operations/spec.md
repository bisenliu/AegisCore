## ADDED Requirements

### Requirement: user-service Swagger UI 依赖升级验证

系统 MUST 将 user-service 的 Swagger UI 静态资源依赖维护在 `github.com/swaggo/files/v2`，并通过交付验证确认 v2 模块路径、embedded `fs.FS` 运行时路由和 OpenAPI 生成链路一致。升级 MUST NOT 保留 `github.com/swaggo/files` v1 依赖、`github.com/swaggo/gin-swagger` wrapper、旧 import、旧 handler fallback 或双版本兼容代码。

#### Scenario: 依赖使用 v2 模块路径

- **WHEN** 协作者审查 `user-service/go.mod`
- **THEN** `user-service` MUST 显式依赖 `github.com/swaggo/files/v2`
- **AND** `user-service` MUST NOT 继续依赖 `github.com/swaggo/files` v1 模块路径

#### Scenario: 编译和测试覆盖升级

- **WHEN** 协作者完成 Swagger UI v2 升级实现
- **THEN** `go test ./user-service/internal/router` MUST 通过
- **AND** 测试 MUST 验证 OpenAPI JSON、OpenAPI UI 或 docs redirect 的当前稳定行为

#### Scenario: OpenAPI 生成链路保持可验证

- **WHEN** 协作者执行 `make user-service-openapi-generate`
- **THEN** 系统 MUST 继续生成 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`
- **AND** 生成链路 MUST NOT 因 Swagger UI v2 依赖升级改变 `tools/openapi-convert` CLI 参数、服务脚本传入参数或输出文件集合

#### Scenario: 完整验证暴露依赖或生成物 drift

- **WHEN** 协作者准备完成本 change
- **THEN** 协作者 MUST 先暂存本次预期代码、OpenSpec artifacts 和必要生成物变更
- **AND** `make lint` 和 `make verify` MUST 通过
- **AND** `make verify` 的最终 `git diff --exit-code` MUST 能暴露未纳入暂存区的依赖、代码或生成物 drift
