# AegisCore

AegisCore 是 Go 1.26 workspace 后端项目底座，当前包含跨服务共享模块 `common` 和用户服务模块 `user-service`。用户服务提供用户资料、认证会话、角色、权限、RBAC 授权、健康探针、OpenAPI、metrics/tracing/logging 和 Ent/Atlas migration 工作流。

## 仓库布局

| 路径 | 目的 |
|---|---|
| `common/` | 跨服务稳定契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力 |
| `user-service/` | 用户服务 CLI、Fx/Gin runtime、feature 业务代码、Ent schema、migrations、OpenAPI 生成 |
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
ADMIN_PASSWORD='<password>' make user-service-create-super-admin
```

数据库和生成物：

```bash
make user-service-generate
make user-service-migrate-diff name=<name>
make user-service-migrate-validate
DATABASE_URL='<postgres-url>' make user-service-migrate-apply
make user-service-openapi-generate
```

Docker 镜像：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
docker build -f deployments/docker/user-service-migration.Dockerfile -t aegiscore-user-services-migration .
```

## OPSX 工作流

稳定边界位于 `openspec/specs/`。跨 feature、跨模块、外部契约、schema、部署或行为变更应先提出新的 change：

```text
/opsx:propose <change-name>
/opsx:apply <change-name>
```

OpenSpec 主规格、change artifacts 和 OPSX 相关文档必须使用简体中文；正文、标题、需求、场景、任务和面向协作者的说明不得保留英文模板内容，技术标识符、路径、命令和 Go symbol 可保留英文原文。

## 发布顺序

生产发布应在 HTTP rollout 前执行数据库迁移和 RBAC seed：

1. 使用专用 Atlas/migration 镜像或 CI/CD release job 对 user-service `user_db` 执行 Atlas migration。
2. 执行 `make user-service-seed-rbac` 初始化 RBAC 系统数据，按需通过 `ADMIN_PASSWORD='<password>' make user-service-create-super-admin` 创建或复用超级管理员账号。
3. 启动或滚动更新 user-service HTTP 副本。

普通服务容器不包含 Atlas，也不执行 migration；`RUN_MIGRATIONS=true` 在运行时镜像中已废弃。简单部署也应先运行 migration 镜像，成功后再启动服务镜像。
