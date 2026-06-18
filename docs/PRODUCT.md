# Product Context

## Goal

AegisCore 是面向多服务后端能力的 Go 项目底座。当前仓库已具备用户服务和共享基础设施，目标是为用户资料、认证会话、RBAC 管理、运行时配置、数据库连接、HTTP 中间件、观测和 API 响应约定提供稳定、可演进的后端基础。

## Users And Roles

| 角色 | 目标 | 关注点 |
|---|---|---|
| 后端开发者 | 快速新增服务能力和 HTTP API | 清晰分层、可复用基础设施、低重复代码 |
| API 调用方 | 通过稳定 API 完成认证、用户、角色、权限操作 | 响应结构一致、错误码可预测、授权语义清楚 |
| 运维/平台人员 | 配置、启动、观测和发布服务 | 配置可覆盖、健康检查、结构化日志、迁移可控、可观测性可接入 |
| AI 代理 | 基于 `AGENTS.md`、docs 和 OpenSpec 安全实施变更 | feature 边界清楚、规格可检索、任务可验证 |

## Current Core Scenarios

- API 调用方登录、刷新 token、强制改密、退出当前设备或全部设备。
- 授权用户创建、查询和分页列出用户资料。
- 管理角色、权限、用户角色绑定和角色权限绑定。
- 查询用户有效权限，并通过 route diff 诊断已注册路由和权限目录差异。
- HTTP 请求经过 OTel tracing、metrics、日志、panic recovery、CORS、JWT 和 RBAC 授权链路。
- 运维系统调用 `/livez`、`/readyz`、`/startupz` 判断进程、流量就绪和启动依赖状态。
- 数据库 schema 变更通过 Ent schema 与 Atlas SQL migration 生成、审查、校验，并在发布流程中显式执行。

## Stable Capabilities

- 用户资料管理。
- 认证会话生命周期。
- 角色管理与绑定。
- 权限目录与有效权限查询。
- RBAC HTTP 授权与多实例 policy sync。
- 统一 HTTP API response/error/pagination 契约。
- 共享 runtime/config/logger/datastore/workerpool/scheduler/observability primitive。
- 健康探针、metrics、tracing 和日志关联。
- Ent/Atlas migration 工作流。
- Docker/Compose 和观测部署资产。

## Product Constraints

- 主规格描述当前已存在或明确保留的稳定能力，不承诺未实现的支付、订单、外部系统调用、MQ、eventbus、outbox 或入站 gRPC API。
- 用户对外标识为 UUID `user_id`，内部数字 `id` 不对外暴露。
- HTTP API 必须使用统一 JSON 响应信封。
- 服务启动依赖 Redis 和 PostgreSQL；`/readyz` 与 `/startupz` 检查关键依赖和 RBAC policy 状态。
- 数据库结构变更必须通过可审查 SQL migration 表达，普通服务容器默认不执行 migration。
