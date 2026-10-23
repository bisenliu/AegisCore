## Context

本地 Compose 默认启动 PostgreSQL、Redis、Nacos、Jaeger、user-service、Prometheus 和 Grafana。Prometheus/Grafana 仅在查看指标和看板时需要；`rbac-seed` 与 `user-service` 则对同一个本地镜像重复声明构建。仓库当前有意使用上游 `latest` 标签，本变更保持这一约定。

## Goals / Non-Goals

**Goals:**

- 默认启动核心服务和 Jaeger；通过 `observability` profile 按需启动 Prometheus/Grafana。
- `rbac-seed` 复用 `user-service` 已构建的镜像，减少重复构建声明。
- 使 Compose 注释和开发文档准确说明启动方式、端口与镜像复用前提。

**Non-Goals:**

- 不固定或迁移上游镜像版本，不清理现有镜像、容器或数据卷。
- 不改变 Go 代码、HTTP API、数据库 migration、OpenAPI 生成物、生产部署清单、观测配置资产或安全边界。

## Decisions

1. Prometheus 与 Grafana 均加入 `observability` profile。这样默认 `up` 不会启动这两个服务，而 `--profile observability up` 仍沿用原来的依赖、端口、卷和配置。备选方案是拆分第二份 Compose 文件，但会复制服务配置并增加维护成本。
2. Jaeger 保持默认服务。当前 user-service 的 tracing 配置和 Compose 依赖以 Jaeger 为本地 OTLP 端点；把它一起移入 profile 会改变默认追踪链路。备选方案是将整套观测服务都设为可选，本次不采用。
3. 仅从 `rbac-seed` 移除 `build`，保留同名 `image: aegiscore-user-service:latest`。先启动或构建 user-service 后运行 seed，使用相同本地镜像。Nacos 的 host/docker 两个初始化任务继续各自声明构建，以保持单独指定其中一个服务时可用。备选方案是共享全部构建声明，但单独运行初始化任务时可能尝试拉取未发布的本地镜像。
4. `common/`、`user-service/` 无需同步代码；变更只涉及 `deployments/compose/` 的编排与说明，以及 `docs/openspec` 中相关运维规格。

## Risks / Trade-offs

- [本地原有 Prometheus/Grafana 容器仍在运行] → Compose profile 只影响后续服务选择；需要减少当前占用时由使用者显式停止这两个容器，不自动删除数据。
- [独立执行 seed 时 user-service 镜像尚未构建] → 文档明确先执行默认 `up --build`，再运行 seed；必要时单独构建 `user-service`。
- [使用 `latest` 导致拉取结果随时间变化] → 保持仓库既有本地开发约定，并通过 Compose 渲染和现有验证门禁检查本次配置；不把本次变更解释为版本升级。

## Migration Plan

更新后默认使用 `docker compose -f deployments/compose/docker-compose.yml up -d --build`。需要 Prometheus/Grafana 时使用 `--profile observability up -d`。回滚时可移除这两个 profile 声明并恢复 seed 的构建声明；现有卷和镜像无需迁移。

验证 `docker compose config`、默认及 profile 服务列表、OpenSpec 校验、部署资产检查和仓库门禁。无需运行数据库 migration、生成 Ent 或 OpenAPI。

## Open Questions

无。
