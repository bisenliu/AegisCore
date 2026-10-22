## Why

本地 Compose 使用的 Nacos 3.2.4 不再响应旧的 `/nacos/actuator/health` 健康检查，导致 Nacos 被判定为 unhealthy，依赖服务无法启动。新数据卷也不再内置管理员密码，配置发布任务随后因登录失败而退出。

## What Changes

- 使用 Nacos 3.x readiness 接口检查服务状态。
- 在发布本地配置前初始化新数据卷的管理员密码；已有管理员时校验现有凭据，不重置密码。
- 更新 Compose 文档中的首次启动和本地凭据说明。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`: 明确 Compose 中 Nacos 健康检查、首次鉴权初始化和配置发布的启动顺序。

## Impact

影响 `deployments/compose/docker-compose.yml`、`deployments/compose/README.md` 和本地 Nacos 数据卷的首次启动流程。不改变用户 API、数据库 schema 或生产部署；SQL migration 仍由受控流程执行。
