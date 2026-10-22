## Context

本地 Compose 跟随 `nacos/nacos-server:latest`。当前镜像为 Nacos 3.2.4，旧健康检查返回 404；Nacos 自 2.4.0 起也不再预置管理员密码。两个配置发布任务依赖 Nacos healthy，随后必须能登录受保护的 v3 Admin API。

## Goals / Non-Goals

**Goals:**

- 让新旧 Nacos 数据卷都能按确定顺序完成 Compose 启动。
- 保留现有 Client、Admin API 和 Console API 鉴权及本地 loopback 端口边界。

**Non-Goals:**

- 不修改 Go 服务代码、HTTP API、数据库 migration、OpenAPI 生成物、Kubernetes/Helm 清单或观测资产。
- 不自动执行 SQL migration，也不重置已有 Nacos 管理员密码。

## Decisions

### Decision: 使用 Nacos 3.x readiness 接口

Compose 健康检查请求公开的 `/nacos/v3/admin/core/state/readiness`。该接口直接表示服务可读状态，且 Nacos 3.2.4 返回 HTTP 200。继续使用旧 Actuator 路径会持续返回 404；仅检查 TCP 端口无法确认服务已经就绪。

### Decision: 独立的一次性鉴权初始化任务

`nacos-init-auth` 复用本地 Nacos 镜像中的 `sh` 和 `curl`，先用环境变量提供的凭据尝试登录。登录成功时直接结束；失败时调用仅允许首次设置管理员密码的官方 API，再次登录确认结果。两个配置发布任务在它成功后运行，避免并发初始化竞态。把该逻辑分别放入两个 seed 工具会重复实现，并扩张仓库级配置发布工具的职责。

## Risks / Trade-offs

- 已有数据卷的密码与 `NACOS_PASSWORD` 不一致时初始化任务失败；保留数据和原密码，由运维提供正确凭据。
- Compose 的默认密码仅用于绑定 loopback 的本地开发；受控环境必须提供独立凭据。

## Migration Plan

更新 Compose 文件后运行 `docker compose -f deployments/compose/docker-compose.yml up -d`。新数据卷自动完成管理员初始化；已有数据卷验证凭据后继续发布配置。回滚时恢复旧 Compose 文件不会删除数据卷，但 Nacos 3.2.4 下的旧健康检查仍会失败。通过 `config --quiet`、重复 `up -d`、`ps -a`、Nacos readiness 和配置任务退出码验证。

## Open Questions

无。
