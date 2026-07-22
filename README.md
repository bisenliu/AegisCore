# AegisCore

AegisCore 是 Go 1.26 workspace 后端项目底座，当前包含跨服务共享模块 `common`、用户服务模块 `user-service` 和仓库级交付工具 `tools`。用户服务提供用户资料、认证会话、角色、权限、RBAC 授权、健康探针、OpenAPI、metrics/tracing/logging 和 Ent/Atlas migration 工作流。

## 仓库布局

| 路径 | 目的 |
|---|---|
| `common/` | 跨服务稳定契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力 |
| `user-service/` | 用户服务 CLI、Fx/Gin runtime、feature 业务代码、Ent schema、migrations、OpenAPI 生成 |
| `tools/` | 仓库级交付工具，例如跨服务复用的 OpenAPI 转换 CLI |
| `deployments/` | Docker、Compose、Kubernetes、Helm、Prometheus/Grafana 部署和观测资产 |
| `docs/` | 当前有效架构、开发、产品、测试、lint 和 runbook 文档 |
| `docs/opsx/` | OPSX 能力地图和变更工作流 |
| `openspec/` | 当前有效 OpenSpec 主规格和后续 change artifacts |

## 入口文档

- 代理和协作者规则：`AGENTS.md`
- 架构说明：`docs/ARCHITECTURE.md`
- 开发说明：`docs/DEVELOPMENT.md`
- 产品上下文：`docs/PRODUCT.md`
- 测试说明：`docs/TESTING.md`
- OPSX 能力地图：`docs/opsx/CAPABILITY_MAP.md`
- OpenSpec 主规格：`openspec/specs/`

## 常用命令

```bash
make help
make build
make test
make lint
make verify
make user-service-run
make user-service-seed-rbac
ADMIN_BOOTSTRAP_PASSWORD='<temporary-password>' ADMIN_USERNAME='initial-admin' make user-service-bootstrap-super-admin
```

数据库和生成物：

```bash
make user-service-generate
make user-service-migrate-diff name=<name>
make user-service-migrate-validate
make user-service-openapi-generate
```

Docker 镜像：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-service .
```

## 运行时配置边界

共享 `common/runtime/config.Config` 只包含 `app`、`server`、`log` 和 `observability`。Redis/PostgreSQL 连接类型由 `common/runtime/resources` 复用，但具名资源及其 `resources.*` 路径由具体服务声明；user-service 当前只声明 `resources.redis.cache_redis` 和 `resources.postgres.primary_db`。认证 token version cache 与 RBAC user role cache 分别位于 `auth.token_version_cache` 和 `rbac.user_role_cache`，不属于共享核心配置。

日志只写 stdout/stderr，采集、保留和轮转由运行平台负责。Tracing 启用后固定通过 OTLP 导出；pprof 使用独立的 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED`、`AEGISCORE_OBSERVABILITY_PPROF_ADDR` 诊断监听，默认关闭。反向代理信任由 Ingress、gateway 或 service mesh 入口边界管理，应用不接受 trusted proxy 配置。进程时区使用平台标准 `TZ`。

## OPSX 工作流

稳定边界位于 `openspec/specs/`。跨 feature、跨模块、外部契约、schema、部署或行为变更应先提出新的 change：

```text
/opsx:propose <change-name>
/opsx:apply <change-name>
```

OpenSpec 主规格、change artifacts 和 OPSX 相关文档必须使用简体中文；正文、标题、需求、场景、任务和面向协作者的说明不得保留英文模板内容，技术标识符、路径、命令和 Go symbol 可保留英文原文。

## 发布顺序

生产发布应在 HTTP rollout 前完成数据库 SQL migration、RBAC seed 和一次性超级管理员 bootstrap：

1. 按 Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行的流程，确认 user-service `primary_db` 已完成本 release 对应 SQL migration；`CREATE EXTENSION IF NOT EXISTS pg_trgm;` 可能需要 DBA 权限或前置动作。
2. 执行 `make user-service-seed-rbac` 初始化 RBAC 系统数据。
3. 在全新数据库上执行 `ADMIN_BOOTSTRAP_PASSWORD='<temporary-password>' ADMIN_USERNAME='initial-admin' ADMIN_NICKNAME='Initial Administrator' make user-service-bootstrap-super-admin` 一次性创建初始超级管理员。
4. 启动或滚动更新 user-service HTTP 副本。
5. 初始管理员使用临时密码登录并完成强制改密。

普通服务容器不包含 Atlas，也不执行 migration；`RUN_MIGRATIONS=true` 在运行时镜像中已废弃。简单部署也应先确认 SQL migration 已受控执行，再启动服务镜像。
