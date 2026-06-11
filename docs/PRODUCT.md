# Product Context

## 1. Project Goal

AegisCore 是面向多服务后端能力的 Go 项目底座。当前仓库已具备用户服务和共享基础设施，目标是为后续账户、支付或其他服务提供统一的运行时、配置、数据库连接、HTTP 中间件和 API 响应约定。

## 2. Users And Roles

| 角色 | 目标 | 关注点 |
|---|---|---|
| 后端开发者 | 快速新增服务能力和 HTTP API | 清晰分层、可复用基础设施、低重复代码 |
| API 调用方 | 通过稳定 API 获取用户资料 | 响应结构一致、错误码可预测 |
| 运维/平台人员 | 配置、启动、观测和发布服务 | 配置可覆盖、健康检查、结构化日志、优雅关闭、迁移可控 |
| AI 代理 | 基于 `AGENTS.md` 和架构文档安全实施变更 | feature 边界清楚、代码位置明确、任务可验证 |

## 3. Current Core Scenarios

1. 服务消费者调用 `GET /api/v1/users/:user_id` 获取用户资料，或调用 `GET /api/v1/users` 分页获取用户列表。
2. 运维系统调用 `/healthz` 判断用户服务是否在线。
3. 服务进程通过配置文件和环境变量加载 HTTP、日志、Redis、Postgres 参数。
4. HTTP 请求经过 trace-id、日志、panic recovery 和 CORS 中间件。
5. 业务错误被映射为统一的 API 响应信封和错误码。
6. 数据库 schema 变更通过 Ent schema 和 Atlas SQL migration 生成、审查、校验并在服务启动前执行。

## 4. Stable Capabilities

- 用户资料查询。
- HTTP 服务运行时。
- 共享基础设施装配。
- API 响应契约。
- 数据库 schema 迁移工作流。
- Go 工具链基线。

## 5. Product Constraints

- 当前仓库仍处于基础服务阶段，已有 API 较少，主规格应描述已存在的稳定能力而非愿景功能。
- 用户数据由 PostgreSQL 中 Ent `User` schema 表达，`username` 唯一，内部 `id` 不对外暴露，对外使用 UUID `user_id`。
- API 调用方依赖统一 JSON 响应结构，新增接口应保持兼容。
- 服务启动依赖 Redis 和 PostgreSQL 可用，健康检查目前只代表 HTTP 进程在线，不代表所有下游依赖健康。
- 数据库结构变更必须通过可审查的 SQL migration 表达，运行时不自动修改 schema。

## 6. Non-Goals For Current Baseline

- 不描述尚未实现的认证、注册、支付或管理后台能力。
- 不把 Ent 生成代码细节当作产品能力。
- 不为当前不存在的 API 承诺外部行为。
