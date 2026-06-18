# AegisCore Agent Guide

本文件为 AI 代理和协作者提供仓库导航。进入任务前，先确认需求属于哪个 capability，再决定是否需要 OpenSpec change。

## 1. 快速入口

- 架构说明：`docs/ARCHITECTURE.md`
- 开发说明：`docs/DEVELOPMENT.md`
- 产品上下文：`docs/PRODUCT.md`
- 测试说明：`docs/TESTING.md`
- 能力地图：`docs/opsx/CAPABILITY_MAP.md`
- OPSX 工作流：`docs/opsx/CHANGE_WORKFLOW.md`
- OpenSpec 配置：`openspec/config.yaml`
- 主规格基线：`openspec/specs/`

## 2. 仓库地图

| 路径 | 作用 |
|---|---|
| `common/` | 跨服务共享契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力 |
| `user-service/` | 用户服务 CLI、Fx/Gin runtime、feature 业务代码、Ent schema、migration 和 OpenAPI 生成 |
| `deployments/` | Docker、Compose、Kubernetes、Helm、Prometheus 和 Grafana 部署观测资产 |
| `docs/` | 架构、开发、产品、测试和 OPSX 工作流说明 |
| `openspec/specs/` | 当前有效主规格，描述长期稳定 capability |
| `openspec/changes/` | 待实施或待归档的 proposal、design、spec delta 和 tasks |

## 3. 关键入口

- 服务 CLI：`user-service/cmd/main.go`，包含 `serve` 和 `rbac` 子命令。
- HTTP 路由：`user-service/internal/router/router.go`，挂载健康检查、OpenAPI、metrics、pprof、认证、权限、角色和用户 API。
- 认证路由：`user-service/internal/features/auth/transport/http/routes.go`。
- 权限路由：`user-service/internal/features/permission/transport/http/routes.go`。
- 角色路由：`user-service/internal/features/role/transport/http/routes.go`。
- 用户路由：`user-service/internal/features/user/transport/http/routes.go`。
- RBAC seed：`user-service/cmd/rbac.go` 和 `user-service/internal/features/role/application/seed/`。

## 4. 常用命令

```bash
make help
make build
make test
make lint
make verify
make user-service-run
make user-service-architecture-lint
```

生成和数据库：

```bash
make user-service-generate
make user-service-migrate-diff name=<migration-name>
make user-service-migrate-validate
DATABASE_URL='<postgres-url>' make user-service-migrate-apply
make user-service-openapi-generate
```

RBAC 引导：

```bash
make user-service-seed-rbac
ADMIN_PASSWORD='<password>' make user-service-create-super-admin
```

## 5. OPSX 工作方式

1. 先看 `docs/opsx/CAPABILITY_MAP.md`，确认需求对应的 capability。
2. 跨 feature、跨模块、外部契约、schema、部署或行为变更必须先创建 OpenSpec change。
3. 用 `/opsx:explore` 梳理问题或方案。
4. 用 `/opsx:propose <change-name>` 创建 proposal、design、spec delta 和 tasks。
5. 准备实现时运行 `/opsx:apply <change-name>`。
6. 实现验证后运行 `/opsx:archive <change-name>` 合并主规格。

如果新仓库或工作区缺少 `openspec/`，先执行：

```bash
openspec init --tools none --force
```

## 6. 语言和文档规则

- OpenSpec 主规格、change artifacts 和 OPSX 相关文档正文使用简体中文。
- 技术术语、路径、命令、配置项名称、HTTP 方法、Go symbol 和 OpenSpec 关键字可保留英文原文。
- 不要保留默认英文模板说明，例如占位注释或未填写的示例文字。
- 更新能力行为时，同步更新 `openspec/specs/<capability>/spec.md` 或对应 change delta。

## 7. 架构边界

- `common/` 不依赖 `user-service/internal/features/`。
- `user-service/internal/shared/` 不引入 Gin、Ent、Redis、Fx、runtime config/logger/datastore 或 HTTP response envelope。
- feature 内保持 domain、application、infrastructure、transport 分层；application/domain/infrastructure 不导入 feature HTTP transport DTO 或 controller。
- 共享用户状态和身份错误位于 `user-service/internal/shared/identity/`。
- 新 capability 优先以业务语义命名，避免以目录名或重构动作命名。

## 8. 高风险区域

- 认证和 token version：`user-service/internal/features/auth/`、`common/security/auth/`。
- RBAC 和 Casbin：`user-service/internal/features/permission/`、`user-service/internal/features/role/`、`common/security/casbin/`。
- 数据库 schema 和 migration：`user-service/ent/`、`user-service/migrations/`。
- OpenAPI 生成物：`user-service/docs/openapi.go`、`user-service/docs/openapi.json`、`user-service/docs/openapi.yaml`。
- 观测资产：`common/runtime/observability/`、`deployments/observability/`、`deployments/compose/grafana/`。
- 发布顺序：先 migration，再 RBAC seed，最后滚动 HTTP 副本。

## 9. 交付检查

- 文档或规格变更：运行 `make user-service-architecture-lint`。
- API 或注解变更：运行 `make user-service-openapi-generate` 并检查 diff。
- Ent schema 变更：运行 `make user-service-generate`、migration diff 和 migration validate。
- 普通代码变更：优先运行相关包测试；合并前运行 `make verify`。
