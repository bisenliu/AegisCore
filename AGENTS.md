# AegisCore Agent Guide

本文件为 AI 代理和协作者提供仓库导航。进入任务前，先确认需求属于哪个 capability，再决定是否需要 OpenSpec change。结构规则以本文件、`docs/ARCHITECTURE.md` 和 `openspec/specs/` 的当前有效主规格为准。

## 1. 快速入口

- 架构说明：`docs/ARCHITECTURE.md`
- 开发说明：`docs/DEVELOPMENT.md`
- 产品上下文：`docs/PRODUCT.md`
- 测试说明：`docs/TESTING.md`
- Lint 自动化：`docs/GO_LINT_AUTOMATION.md`
- 能力地图：`docs/opsx/CAPABILITY_MAP.md`
- OPSX 工作流：`docs/opsx/CHANGE_WORKFLOW.md`
- OpenSpec 配置：`openspec/config.yaml`
- 主规格基线：`openspec/specs/`

## 2. 仓库地图

| 路径 | 作用 |
|---|---|
| `go.work` | Go workspace，包含 `common` 和 `user-service` 两个模块；仓库根目录不是单一 Go module |
| `common/` | 跨服务共享契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力 |
| `user-service/` | 用户服务 CLI、Fx/Gin runtime、feature 业务代码、Ent schema、migration 和 OpenAPI 生成 |
| `deployments/` | Docker、Compose、Kubernetes、Helm、Prometheus 和 Grafana 部署观测资产 |
| `docs/` | 架构、开发、产品、测试和 OPSX 工作流说明 |
| `openspec/specs/` | 当前有效主规格，描述长期稳定 capability |
| `openspec/changes/` | 待实施或待归档的 proposal、design、spec delta 和 tasks |

`common/` 只承载跨服务稳定能力：`contract/errors`、`contract/pagination`、`contract/response`、`http/binding`、`http/middleware`、`http/openapi`、`http/pprof`、`http/response`、`runtime/config`、`runtime/datastore`、`runtime/id`、`runtime/localcache`、`runtime/logger`、`runtime/observability`、`runtime/rediskey`、`runtime/resources`、`runtime/scheduler`、`runtime/timezone`、`runtime/workerpool`、`security/auth`、`security/casbin`、`security/password`、`testing` 和 `validation`。

`user-service/internal/` 的主要边界：

- `bootstrap/`：顶层 Fx app 与 HTTP server lifecycle。
- `providers/`：服务级 Fx provider，组装 Gin、routes、JWT、PostgreSQL/Redis named resources、Ent clients、metrics、tracing 和 health checks，不承载 feature 业务逻辑。
- `router/`：健康探针、OpenAPI、metrics、pprof 和 `/api/v1` feature route graph 总装。
- `features/user|auth|role|permission/`：按 feature 分层组织业务代码。
- `shared/identity`：user/auth 共同消费的用户状态、账号生命周期判断和身份错误。
- `shared/rbacbaseline`：role/permission 共同消费的系统 RBAC 角色、权限和默认绑定规格。
- `integration/http|grpc|events`：真实外部系统协议适配和防腐层；当前无真实外部 client、入站 gRPC API、MQ/broker、eventbus 或 outbox。

## 3. 关键入口

- 服务 CLI：`user-service/cmd/main.go`，包含 `serve` 和 `rbac` 子命令。
- RBAC CLI：`user-service/cmd/rbac.go`，包含 `seed`、`assign-super-admin` 和 `create-super-admin`。
- 服务组装：`user-service/internal/bootstrap/app.go`、`user-service/internal/bootstrap/server.go`、`user-service/internal/providers/fx.go`。
- HTTP 路由：`user-service/internal/router/router.go`，挂载健康检查、OpenAPI、metrics、pprof、认证、权限、角色和用户 API。
- 健康、metrics、OpenAPI：`user-service/internal/router/health.go`、`metrics.go`、`openapi.go`。
- 认证路由：`user-service/internal/features/auth/transport/http/routes.go`。
- 权限路由：`user-service/internal/features/permission/transport/http/routes.go`。
- 角色路由：`user-service/internal/features/role/transport/http/routes.go`。
- 用户路由：`user-service/internal/features/user/transport/http/routes.go`。
- RBAC seed：`user-service/internal/features/role/application/seed/`，系统基线来自 `user-service/internal/shared/rbacbaseline/`。
- Ent schema 和 migration：`user-service/ent/schema/`、`user-service/migrations/atlas.hcl`、`user-service/migrations/*.sql`。
- OpenAPI 生成：`user-service/scripts/openapi-generate.sh`、`user-service/docs/openapi.go`、`openapi.json`、`openapi.yaml`。

## 4. 当前能力

- 用户资料：`POST /api/v1/users`、`GET /api/v1/users/:id`、`GET /api/v1/users`。
- 认证会话：登录、refresh、强制改密、退出当前会话、退出全部会话、refresh session 上限和 token version 撤销校验。
- 权限目录：权限创建、更新、启停、查询、有效权限和 route diff 诊断。
- 角色管理：角色创建、更新、启停、查询、角色权限绑定和用户角色绑定。
- RBAC 授权：JWT 认证后使用 Casbin 对用户、角色和权限业务接口授权；在线 RBAC 写操作通过本实例 reload、Redis policy version、Pub/Sub 和定时补偿同步其他副本。
- 运行时观测：`/livez`、`/readyz`、`/startupz`、配置化 metrics endpoint、OpenAPI UI/JSON、pprof、Prometheus alerts 和 Grafana dashboards。

## 5. 常用命令

```bash
make help
make build
make test
make lint
make verify
make user-service-build
make user-service-run
make user-service-architecture-lint
make common-test
make user-service-test
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
make user-service-create-super-admin
ADMIN_PASSWORD='<password>' make user-service-create-super-admin
ADMIN_PASSWORD='<password>' ADMIN_RESET_PASSWORD=true make user-service-create-super-admin
```

根 `Makefile` 中服务私有目标必须带服务名前缀，例如 `user-service-seed-rbac`，不要新增 `seed-rbac` 这类无服务上下文的根目标。

## 6. OPSX 工作方式

1. 先看 `docs/opsx/CAPABILITY_MAP.md`，确认需求对应的 capability。
2. 小改动可以直接实现；跨 feature、跨模块、目录结构、外部契约、schema、部署或稳定行为变更必须先创建 OpenSpec change。
3. 用 `/opsx:explore` 梳理问题或方案。
4. 用 `/opsx:propose <change-name>` 创建 proposal、design、spec delta 和 tasks。
5. 准备实现时运行 `/opsx:apply <change-name>`。
6. 实现验证后运行 `/opsx:archive <change-name>` 合并主规格。

如果新仓库或工作区缺少 `openspec/`，先执行：

```bash
openspec init --tools none --force
```

## 7. 语言和文档规则

- OpenSpec 主规格、change artifacts 和 OPSX 相关文档正文、标题、需求、场景、任务和说明使用简体中文。
- 技术术语、路径、命令、配置项名称、HTTP 方法、Go symbol、错误码、数据库字段和 OpenSpec 关键字可保留英文原文。
- 不要保留默认英文模板说明，例如占位注释或未填写的示例文字。
- 更新能力行为时，同步更新 `openspec/specs/<capability>/spec.md` 或对应 change delta。
- 代码注释和函数/方法注释使用中文；log message 使用英文，日志字段名使用稳定英文 `snake_case`。

## 8. 架构边界

- `common/` 不依赖 `user-service/internal/features/`，也不承载 user-service 业务 DTO、feature key schema、policy loader、route diff、OpenAPI 服务元数据、eventbus/outbox 设计或推测性 helper。
- `common/http/openapi` 只承载 Swagger/OpenAPI 转换、规范化、序列化和 Go embed 渲染 helper；服务 API server、认证方案、扫描范围和输出目录由服务脚本或薄 wrapper 拥有。
- `common/security/casbin` 只承载通用请求三元组和 authorizer 包装；user-service subject schema、权限目录、策略加载、超级管理员基线和 route diff 留在 permission/shared 边界。
- `common/runtime/workerpool`、`scheduler`、`rediskey`、`localcache` 只提供无业务语义 primitive；auth/user/role/permission 的 key schema、缓存策略和安全语义留在对应 feature。
- `user-service/internal/shared/` 只允许至少两个 feature 真实消费、边界稳定且不能归入 `common` 的服务内业务内核；当前只开放 `identity` 与 `rbacbaseline`。
- `user-service/internal/shared/` 不引入 feature 包、Gin、Ent、Redis、SQL、Fx、runtime config/logger/datastore、HTTP response envelope、controller、DTO、store port、use case、外部调用或部署资产。
- feature 内保持 domain、application、transport、infrastructure 分层；application/domain/infrastructure 不导入 feature HTTP transport DTO 或 controller。
- HTTP controller 先用 `binding.BindOrAbort`，再调用 feature-local input preparer 进行裁剪、默认值归一化、UUID/cursor/token 解析和 command/query 构造；input preparer 不查询 store、不调用 use case、不执行授权、不写 HTTP 响应。
- Ports 由消费侧 feature application 拥有，infrastructure adapter 只实现这些最小接口；不要为了 adapter 方便在 infrastructure 包或共享根包定义大接口。
- `transport/grpc`、`domain/events`、`domain/services` 和 `infrastructure/consumers` 只有存在真实 API、领域事件模型、纯领域服务或消费者需求时才承载业务代码；当前没有真实 gRPC API、MQ/broker、eventbus、outbox、producer、subscriber、consumer handler 或后台投递 worker。
- 外部系统防腐层统一使用 `user-service/internal/integration/http|grpc|events`；`integration/grpc` 是出站 external client adapter，不是本服务入站 gRPC transport。
- 不要手写 `user-service/ent/` 生成代码或 OpenAPI 生成物；通过 `make user-service-generate` 和 `make user-service-openapi-generate` 更新。
- 运行时服务代码不得使用 `client.Schema.Create(ctx)` 表达 schema 变更；Ent schema 变化必须生成 Ent 代码和 Atlas SQL migration。
- Ent predicate 构造封装在 infrastructure adapter 内，application 层不得直接导入 `github.com/aegiscore/user-service/ent/<entity>` predicate 包。

## 9. 高风险区域

- 认证、refresh session、强制改密和 token version：`user-service/internal/features/auth/`、`common/security/auth/`。
- RBAC、policy sync 和 Casbin：`user-service/internal/features/permission/`、`user-service/internal/features/role/`、`user-service/internal/shared/rbacbaseline/`、`common/security/casbin/`。
- 数据库 schema 和 migration：`user-service/ent/schema/`、`user-service/migrations/`。
- OpenAPI 生成物：`user-service/docs/openapi.go`、`user-service/docs/openapi.json`、`user-service/docs/openapi.yaml`。
- 可观测性：`common/runtime/observability/`、`common/runtime/logger/`、`common/http/middleware/`、`deployments/observability/`、`deployments/compose/grafana/`。
- 部署发布：生产优先使用独立 Atlas/migration 镜像的 migration Job 或 CI/CD release job；普通 user-service 运行时镜像不包含 Atlas，不执行 migration，`RUN_MIGRATIONS=true` 已废弃。

## 10. 交付检查

- 文档或规格变更：运行 `make user-service-architecture-lint`。
- API 或注解变更：运行 `make user-service-openapi-generate` 并检查 diff。
- Ent schema 变更：运行 `make user-service-generate`、`make user-service-migrate-diff name=<migration-name>` 和 `make user-service-migrate-validate`。
- 部署观测资产变更：运行 `make compose-dashboard-check` 或对应生成脚本。
- 普通代码变更：优先运行相关包测试；合并前运行 `make verify`。
