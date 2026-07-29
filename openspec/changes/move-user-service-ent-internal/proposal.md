## Why

`user-service/ent/` 当前位于 `user-service` 模块根级，形成可被其他 Go module 直接 import 的公开包路径，无法通过 Go `internal` 规则保护 Ent client、entity、predicate 和 migration 生成物。这会让未来服务绕过 user-service 的 feature、application、domain 和 infrastructure adapter 边界直接耦合数据库模型，增加微服务拆分和 schema 演进风险。

## What Changes

- **BREAKING** 移除模块根级 `github.com/aegiscore/user-service/ent` 公开包路径，不保留 re-export、type alias、shim package 或兼容分支。
- 将 user-service 的 Ent schema、生成代码、`enttest` 和 `migrate` 生成物统一迁入 `user-service/internal/persistence/ent/`。
- 调整 Ent 生成入口和 Atlas schema helper，使后续生成物稳定落在 `internal/persistence/ent` 包路径下。
- 更新 user-service 内部 provider、feature infrastructure、RBAC CLI 和测试代码的 Ent import 路径。
- 增加或更新架构检查，防止重新引入模块根级 `user-service/ent` 公开 import。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`：Ent/Atlas 生成与迁移流程必须使用受 `internal` 保护的 user-service 私有 Ent 包路径。

## Impact

- 影响 Go 代码：`user-service/ent/`、`user-service/internal/providers/`、`user-service/internal/features/*/infrastructure/`、`user-service/cmd/`、`user-service/scripts/atlas-schema/` 和相关测试。
- 影响生成流程：`make user-service-generate` 需要生成到 `user-service/internal/persistence/ent/`，不得重新生成模块根级 `user-service/ent/`。
- 影响迁移验证：Atlas schema helper 需要改用新的 `internal/persistence/ent/migrate` 路径，现有 SQL migration 内容不应因目录迁移发生语义变化。
- 影响安全与架构边界：其他模块不应再能 import user-service 的 Ent client、entity、predicate、`enttest` 或 `migrate` 包。
- 不影响 HTTP API、OpenAPI 契约、数据库 schema 语义、部署清单、Prometheus/Grafana 观测资产或运行时配置。
