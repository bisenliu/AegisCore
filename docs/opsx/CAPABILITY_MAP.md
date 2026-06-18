# OPSX 能力地图

| 能力 | 目的 | 代码区域 | 主规格 |
|---|---|---|---|
| 仓库架构 | Workspace、module、目录归属和禁止漂移规则 | `go.work`, `common/`, `user-service/`, `deployments/` | `openspec/specs/repository-architecture/spec.md` |
| 功能分层 | Feature-first 分层、ports、DTO、adapter 边界 | `user-service/internal/features/*` | `openspec/specs/feature-layering/spec.md` |
| HTTP API 契约 | 响应信封、错误、分页、binding/input flow | `common/contract`, `common/http`, `features/*/transport/http` | `openspec/specs/http-api-contract/spec.md` |
| 用户资料 | 用户创建、详情、列表 | `user-service/internal/features/user` | `openspec/specs/user-profile/spec.md` |
| 认证会话 | 登录、刷新、强制改密、登出、session/token version | `user-service/internal/features/auth`, `common/security` | `openspec/specs/auth-session/spec.md` |
| RBAC | 角色、权限、授权、policy sync、seed workflow | `features/role`, `features/permission`, `internal/shared/rbacbaseline` | `openspec/specs/rbac/spec.md` |
| 共享内核 | `internal/shared` 准入与 `identity`/`rbacbaseline` | `user-service/internal/shared` | `openspec/specs/shared-kernel/spec.md` |
| 公共契约与运行时 | 跨服务契约和 runtime primitive 边界 | `common/contract`, `common/runtime`, `common/security` | `openspec/specs/common-contracts-runtime/spec.md` |
| 可观测性与健康检查 | metrics、tracing、logging、probes、runbook | `common/runtime/observability`, `common/http/middleware`, `user-service/internal/router` | `openspec/specs/observability-health/spec.md` |
| 数据库迁移 | Ent/Atlas schema 变更和 release migration | `user-service/ent/schema`, `user-service/migrations`, `scripts/migrate-*` | `openspec/specs/database-migrations/spec.md` |
| 部署资产 | Docker、Compose、K8s、Helm、观测资产归属 | `deployments/` | `openspec/specs/deployment-assets/spec.md` |
| 测试与质量 | test gates、container/e2e gating、lint/depguard | `Makefile`, `.golangci.yml`, `common/testing`, `.github/` | `openspec/specs/testing-quality/spec.md` |
| OpenAPI 生成 | 生成脚本、common converter、运行时 docs route | `common/http/openapi`, `user-service/scripts/openapi-generate.sh`, `user-service/docs` | `openspec/specs/openapi-generation/spec.md` |
| 外部集成边界 | 出站 HTTP/gRPC/events 防腐层和未来 consumer 边界 | `user-service/internal/integration`, future `infrastructure/consumers` | `openspec/specs/external-integration-boundary/spec.md` |

## 文档语言规则

`openspec/specs/`、`openspec/changes/` 和 `docs/opsx/` 下的主规格、change artifacts、工作流和能力地图必须使用简体中文。后续生成或更新时，正文、标题、需求、场景、任务和面向协作者的说明不得保留英文模板内容；技术标识符、路径、命令和 Go symbol 可保留英文原文。
