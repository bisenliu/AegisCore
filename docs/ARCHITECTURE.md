# AegisCore 架构说明

## 1. 总体结构

AegisCore 是 Go 1.26 workspace，当前由三个主要部分组成：

| 模块 | 职责 |
|---|---|
| `common/` | 跨服务稳定契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力 |
| `user-service/` | 用户服务 CLI、Fx/Gin runtime、feature 业务代码、Ent schema、migration 和 OpenAPI 生成 |
| `deployments/` | Docker、Compose、Kubernetes、Helm、Prometheus 和 Grafana 部署观测资产 |

根 `Makefile` 汇总构建、测试、lint、verify、migration、OpenAPI 和 dashboard 命令。README 将 `docs/` 与 `openspec/` 定义为协作和规格入口。

## 2. `common/` 共享基础

`common/` 是跨服务复用层，不依赖 `user-service` 的 feature 实现。

| 路径 | 职责 |
|---|---|
| `common/contract/errors/` | 统一错误码、错误对象和转换 |
| `common/contract/response/` | 统一响应 envelope 和消息 |
| `common/contract/pagination/` | 分页数据结构和 helper |
| `common/http/binding/` | Gin 请求绑定与校验衔接 |
| `common/http/middleware/` | auth、casbin、cors、logging、metrics、recovery 和 span error |
| `common/http/openapi/` | OpenAPI 转换和渲染 helper |
| `common/http/response/` | HTTP 响应写入和错误响应 |
| `common/runtime/config/` | YAML 配置加载和 validation |
| `common/runtime/datastore/` | Postgres、Redis 和 Fx provider |
| `common/runtime/logger/` | zap logger 和 writer |
| `common/runtime/observability/` | metrics 与 tracing provider |
| `common/runtime/scheduler/` | scheduler、lock、metrics 和 logger |
| `common/runtime/workerpool/` | 固定 worker pool、stats 和 errors |
| `common/security/` | JWT、token version、Casbin authorizer、password hash |
| `common/testing/` | Postgres/Redis Testcontainers 和 fixtures |
| `common/validation/` | validator、翻译、字段和错误 |

## 3. `user-service` 运行入口

`user-service/cmd/main.go` 定义 `aegiscore-user-services` CLI：

- `serve --config <path>`：启动 Fx app 和 Gin HTTP server。
- `rbac seed`：初始化默认系统角色、权限和绑定。
- `rbac assign-super-admin --user-id <uuid>`：为已有用户绑定超级管理员角色。
- `rbac create-super-admin`：创建或复用管理员用户并绑定超级管理员角色。

`user-service/internal/bootstrap/` 构造应用和 HTTP server。`user-service/internal/providers/` 提供 Gin、Ent、Postgres、Redis、auth、metrics、health 和 routes provider。

## 4. HTTP 路由结构

`user-service/internal/router/router.go` 是 HTTP 路由聚合点：

1. 注册健康检查。
2. 注册 OpenAPI。
3. 按配置注册 pprof。
4. 按配置注册 metrics。
5. 挂载 `/api/v1` 业务路由。

业务路由分层：

- `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/auth/change-password` 由认证公开路由挂载。
- 其余受保护路由先通过 `AuthWithTokenVersionValidator`。
- 权限、角色和用户接口再通过 `permissionhttp.Authorize` 执行 RBAC 授权。
- 用户 API 位于 `/api/v1/users`。
- 权限 API 位于 `/api/v1/permissions`。
- 角色 API 位于 `/api/v1/roles` 和 `/api/v1/users/:user_id/roles`。

## 5. Feature 分层

`user-service/internal/features/` 以能力划分 feature：

| Feature | 主要职责 |
|---|---|
| `auth` | 登录、刷新、退出、改密、会话、token、凭证和 token version |
| `permission` | 权限目录、路由差异、有效权限、Casbin policy 和授权中间件 |
| `role` | 角色、角色权限、用户角色、系统 seed 和超级管理员绑定 |
| `user` | 用户资料创建、查询、列表、状态和存储 |

典型 feature 内部结构：

- `domain/`：领域对象、值对象和领域错误。
- `application/`：命令、查询、服务、端口、validator 和 metrics。
- `infrastructure/`：Postgres、Redis、Casbin 等适配器。
- `transport/http/`：controller、request、response、mapper、routes 和输入校验。

分层约束由 `user-service/scripts/architecture-lint.sh` 检查。

## 6. 核心流程

### 6.1 服务启动

1. `aegiscore-user-services serve --config ./configs/config.yaml` 进入 `runServe`。
2. `bootstrap.NewApp(configPath)` 构造 Fx app。
3. provider 初始化配置、logger、datastore、auth、metrics、health、routes 和 HTTP server。
4. 收到 SIGINT 或 SIGTERM 后使用独立 stop timeout 优雅关闭。

### 6.2 登录和会话

1. HTTP 请求进入 `auth/transport/http` controller。
2. application command 校验输入、凭证和用户状态。
3. session store 写入 Redis 会话状态。
4. token issuer 签发 access token 和 refresh token。
5. 响应通过共享 response helper 返回统一 envelope。

### 6.3 受保护 API 授权

1. 请求进入 `/api/v1` authenticated group。
2. JWT、auth config 和 token version validator 校验 bearer token。
3. RBAC 中间件读取当前用户和请求资源。
4. permission authorizer 使用 Casbin 或同步后的 policy 判断访问权限。
5. 通过后进入目标 controller。

### 6.4 RBAC seed 和超级管理员

1. `rbac seed` 加载配置，打开 user DB，创建 Ent client。
2. role seed service 创建或更新系统角色、权限和绑定。
3. `create-super-admin` 读取 `ADMIN_PASSWORD`，创建或复用用户，按需更新密码。
4. seed service 绑定内置超级管理员角色。

### 6.5 数据迁移

1. Ent schema 变化后执行 `make user-service-generate`。
2. 使用 `make user-service-migrate-diff name=<migration-name>` 生成 Atlas migration。
3. 使用 `make user-service-migrate-validate` 校验 migration。
4. 发布时使用 `DATABASE_URL` 执行 `make user-service-migrate-apply`。

## 7. 部署和观测

- Dockerfile：`deployments/docker/user-service.Dockerfile`。
- Compose：`deployments/compose/docker-compose.yml`。
- Kubernetes：`deployments/k8s/user-services/`。
- Helm：`deployments/helm/aegiscore-user-services/`。
- Prometheus alerts：`deployments/observability/prometheus/user-service-alerts.yaml`。
- Grafana dashboard：`deployments/observability/grafana/user-service-overview.json`。
- Compose dashboard 由 `deployments/compose/scripts/generate-grafana-dashboard.sh` 从通用观测 dashboard 生成。

## 8. 规格边界

稳定能力位于 `openspec/specs/`。跨 feature、跨模块、外部契约、schema、部署或行为变更必须先创建 `openspec/changes/<change-name>/`，完成后归档回主规格。
