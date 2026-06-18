# Architecture

## Overview

AegisCore 是 Go 1.26 workspace，当前包含共享基础模块 `common` 和用户服务模块 `user-service`。用户服务通过 Cobra 提供 CLI，通过 Uber Fx 组装依赖，通过 Gin 暴露 HTTP API，通过 Ent 访问 PostgreSQL，并通过 Atlas 维护服务内 SQL migration。

本文件、根目录 `AGENTS.md` 和 `openspec/specs/` 是当前有效结构、分层和能力规格来源。OpenSpec 主规格、change artifacts 和 OPSX 相关文档的正文、标题、需求、场景、任务和说明必须使用简体中文；包名、路径、HTTP method、配置 key、CLI 命令、Go symbol 等技术标识符可保留英文原文。

## Module Boundaries

| 模块 | 责任 | 关键位置 |
|---|---|---|
| `common` | 跨服务稳定契约与无业务语义基础能力 | `common/contract/`, `common/runtime/`, `common/http/`, `common/security/`, `common/testing/`, `common/validation/` |
| `user-service` | 用户服务运行时、业务 feature、服务内 shared kernel、Ent schema、Atlas migration、服务侧 OpenAPI | `user-service/cmd/`, `user-service/internal/`, `user-service/ent/`, `user-service/migrations/`, `user-service/docs/` |
| `deployments` | Docker、Compose、Kubernetes、Helm、Prometheus/Grafana 等部署和观测资产 | `deployments/docker/`, `deployments/compose/`, `deployments/k8s/`, `deployments/helm/`, `deployments/observability/` |

仓库根目录是 workspace，不是 Go module。Go 命令通常通过根 `Makefile` 统一入口执行，或在 `common/`、`user-service/` 内通过模块级 `Makefile` 执行。

## Runtime Flow

1. `user-service/cmd/main.go` 创建 `aegiscore-user-services` CLI，并注册 `serve`、`rbac seed`、`rbac assign-super-admin`、`rbac create-super-admin`。
2. `serve` 调用 `bootstrap.NewApp(configPath)` 创建 Fx 应用。
3. `user-service/internal/bootstrap.AppModule` 导入共享 runtime module、feature modules、`providers.Module` 和 HTTP server lifecycle。
4. `user-service/internal/providers.Module` 提供 Redis/PostgreSQL named resources、Ent clients、JWT service、metrics/tracing provider、Gin engine 和 route registration。
5. User/Auth/Role/Permission feature modules 自己组装 application、transport 和 infrastructure provider。
6. `router.go` 负责 `/api/v1` route graph，总装健康探针、metrics、OpenAPI 和 feature 路由。
7. Fx lifecycle 启动 HTTP server，并在中断或 SIGTERM 时先停止接收新请求，再 drain active handlers，最后关闭底层资源。

## Feature-First Organization

服务内业务代码按 feature 组织在 `user-service/internal/features/<feature>/`。当前稳定 feature 包括：

- `user`：用户资料创建、查询和分页列表。
- `auth`：登录、刷新、强制改密、退出当前设备、退出全部设备，以及 refresh session 生命周期治理。
- `role`：角色生命周期、用户角色绑定、角色权限绑定和角色查询。
- `permission`：权限目录生命周期、有效权限查询、route diff、RBAC authorization wrapper、Gin RBAC middleware、Casbin policy loader/enforcer/reload 和 policy sync。

每个 feature 按以下层组织：

| 层 | 责任 |
|---|---|
| `domain` | 实体、值对象、领域错误和纯规则；`domain/services`、`domain/events` 只在有真实领域规则或事件模型时创建 |
| `application` | command/query/use case、消费侧 ports、业务编排和 transport-neutral validators/components |
| `transport/http` | Gin controller、route registration、HTTP request/response DTO、OpenAPI DTO、input preparer |
| `transport/grpc` | 未来真实入站 gRPC transport；当前没有真实 API 时只允许 README 或 package doc |
| `infrastructure/postgres` | Ent/PostgreSQL adapter、predicate 构造和存储错误转换 |
| `infrastructure/redis` | Redis adapter、feature 私有 key schema 和 Lua/TTL/session 语义 |
| `infrastructure/consumers` | 未来 feature-local 外部事件输入到 application 的 adapter |
| `fx.go` | Feature-local Fx wiring，不承载业务逻辑 |

HTTP controller 输入处理保持两步：先 `binding.BindOrAbort`，再调用一个 feature-local input preparer。Application/use case 不接收 HTTP DTO 或 protobuf DTO。Ports 由消费侧 feature application 拥有，infrastructure adapter 只实现这些最小接口。

## RBAC Boundary

RBAC 授权由 permission feature 拥有，系统 RBAC 基线由 `user-service/internal/shared/rbacbaseline` 拥有。Casbin subject 使用 `user:<user_uuid>` 与 `role:<role_uuid>`，object 使用 Gin route template，action 使用 HTTP method；policy loader 使用角色 UUID，不要求 `roles.code`。

Route diff 是只读诊断能力，只返回 missing/stale 差异，不创建权限、不修改状态、不绑定角色。在线 RBAC 管理接口变更权限、角色状态、用户角色绑定或角色权限绑定后，必须触发本实例 policy reload，并通过 Redis policy version、Pub/Sub 和定时版本补偿同步其他副本。授权热路径不做每请求 Redis 强一致门控。`rbac seed`、`rbac assign-super-admin` 与 `rbac create-super-admin` 是离线运维入口，应在 migration 后、HTTP rollout 前执行；`rbac seed` 不得自动创建真实业务用户或分配超级管理员角色，`rbac create-super-admin` 必须显式执行并通过环境变量读取密码。

## Shared Kernel And Integration

`user-service/internal/shared` 只允许已被至少两个 feature 真实消费、边界稳定且不能归入 `common` 的纯业务规格。当前只开放：

- `identity`：user/auth 共同消费的用户状态、账号生命周期判断和用户身份错误。
- `rbacbaseline`：role/permission 共同消费的系统 RBAC 角色、权限和默认绑定规格。

`internal/shared` 不得导入 feature 包、Gin、Ent、Redis、SQL、Fx、HTTP response、runtime provider，不得承载 controller、DTO、store port、use case、配置读取、日志副作用、外部调用或部署资产。

外部系统防腐层统一在 `user-service/internal/integration/http|grpc|events`。当前没有真实外部 client、入站 gRPC API、MQ/broker、eventbus、outbox、producer/subscriber/consumer handler 或后台投递 worker；不得在没有单独设计时新增这些依赖或持久化模型。

## Dependency Rules

| 层 | 可以依赖 | 禁止依赖 |
|---|---|---|
| `domain` | 标准库、稳定值对象、同 feature domain | Gin、Ent、Redis、config、logger、response envelope、application ports、infrastructure |
| `application` | `domain`、消费侧端口接口、common 安全原语 | Gin、Ent、Redis、HTTP binder |
| `transport/http` | `application`、Gin、response envelope、feature-local DTO/validation | Ent、Redis、SQL |
| `transport/grpc` | `application`、gRPC/protobuf 边界类型、feature-local validation | Ent、Redis、SQL、HTTP response、Gin controller、external client adapter |
| `infrastructure/postgres` | Ent、SQL、application ports、domain | Gin、HTTP response |
| `infrastructure/redis` | Redis client、application ports、domain、common runtime primitive | Gin、HTTP response |
| `infrastructure/consumers` | feature application、feature domain、归一化事件输入 DTO | broker SDK subscription loop、Ent/Redis 直接业务访问、Gin、跨 feature orchestration |
| `integration/*` | 外部 SDK/client、feature application ports、domain、common runtime/security | Gin response、Ent、feature service 业务编排、service-owned persistence adapter |
| `internal/shared/*` | 标准库、稳定无副作用值对象 | feature 包、Gin、Ent、Redis、SQL、Fx、HTTP response、runtime provider、controller、DTO、store port、use case |
| `fx.go` | Fx、feature 内部包 | 业务逻辑 |

## Common Organization

- `common/contract/errors`：稳定错误码和应用错误类型。
- `common/contract/pagination`：Cursor/Keyset 分页契约。
- `common/contract/response`：HTTP 响应信封 DTO。
- `common/http/binding`：Gin 绑定与校验失败响应适配。
- `common/http/response`：Gin 响应输出 helper。
- `common/http/middleware`：无业务语义 Gin middleware 骨架。
- `common/http/openapi`：Swagger/OpenAPI 构建期转换、规范化、序列化和 Go embed 渲染 helper。
- `common/runtime/*`：配置、日志、datastore、资源名、ID、localcache、rediskey、workerpool、scheduler、observability、timezone 等 runtime primitive。
- `common/security/*`：JWT/Bearer/密码/Casbin 等安全原语。
- `common/testing/*`：跨模块测试基础设施和无业务语义 fixture。
- `common/validation`：通用结构校验核心。

`common` 不承载 user-service 业务语义、服务特定 DTO、feature key schema、policy loader、route diff、OpenAPI 服务元数据、eventbus/outbox 设计或 speculative helper。

## Data And Migrations

Ent schema 是数据库结构来源，开发维护文件位于 `user-service/ent/schema/` 和 `user-service/ent/generate.go`。不要手写 `user-service/ent/` 生成代码。SQL migrations 位于 `user-service/migrations/`，Atlas 配置为 `user-service/migrations/atlas.hcl`。运行时不得通过 `client.Schema.Create(ctx)` 表达 schema 变更。生产迁移通过 CI/CD release job 或独立 migration Job 在 HTTP rollout 前执行；容器 `RUN_MIGRATIONS=true` 只适合简单部署或兼容场景。

## Observability And Logging

Metrics 配置位于 `observability.metrics`，tracing 配置位于 `observability.tracing`。用户服务启用 metrics 时暴露配置化 scrape endpoint，默认 `/metrics`，不进入 RBAC 授权，必须由部署或网络侧保护。Tracing 使用 W3C `traceparent` / `tracestate`，logger 从当前 OTel span context 自动追加 `trace_id` 和 `span_id`，无有效 span context 时不得伪造。

代码注释和函数/方法注释使用中文。Log message 使用英文，字段名使用稳定英文 snake_case。不得记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL、Redis key 或敏感原始错误。
