# AegisCore Agent Guide

本文件是 AI 代理和协作者的仓库入口规则。结构规则以本文件和 `docs/ARCHITECTURE.md` 为准；本仓库不再维护 OpenSpec/OPSX 工件，不要重新新增 `openspec/` 或 `docs/opsx/`。

## 1. Quick Start

- 架构与分层规则：`docs/ARCHITECTURE.md`
- 开发说明：`docs/DEVELOPMENT.md`
- 产品上下文：`docs/PRODUCT.md`
- 测试说明：`docs/TESTING.md`
- Lint 自动化：`docs/GO_LINT_AUTOMATION.md`

## 2. Repository Shape

- `go.work`：Go workspace，包含 `common` 和 `user-service` 两个模块。
- `common/`：跨服务稳定契约和基础能力，按 `contract`、`runtime`、`http`、`security`、`validation` 分类组织；不得作为服务特定 helper 的兜底目录。
- `user-service/`：用户服务 HTTP 运行时，包含 Cobra 入口、Fx 组装、Gin 路由、Ent schema、Atlas migration，以及按 feature 组织的业务代码。
- `user-service/internal/features/user/`：用户资料 feature，按 `api/`、`app/`、`domain/`、`transport/http/`、`infra/postgres/` 和 `module.go` 分层。
- `user-service/internal/features/auth/`：认证会话 feature，按 `api/`、`app/`、`domain/`、`transport/http/`、`infra/postgres/`、`infra/redis/` 和 `module.go` 分层。
- `deployments/`：Docker、Compose、Kubernetes 和 Helm 部署资产。

## 3. Key Entry Points

- CLI 入口：`user-service/cmd/main.go`
- 服务组装：`user-service/internal/bootstrap/app.go`
- HTTP 路由总装：`user-service/internal/bootstrap/routes.go`
- Gin router 基础设置：`user-service/internal/router/router.go`
- 用户 feature module：`user-service/internal/features/user/module.go`
- 用户 controller：`user-service/internal/features/user/transport/http/controller.go`
- 用户 service：`user-service/internal/features/user/app/service.go`
- 用户 PostgreSQL adapter：`user-service/internal/features/user/infra/postgres/user_store.go`
- 认证 feature module：`user-service/internal/features/auth/module.go`
- 认证 controller：`user-service/internal/features/auth/transport/http/controller.go`
- 认证 service：`user-service/internal/features/auth/app/service.go`
- 认证 PostgreSQL adapter：`user-service/internal/features/auth/infra/postgres/credential_store.go`
- 认证 Redis adapter：`user-service/internal/features/auth/infra/redis/session_store.go`
- 共享配置加载：`common/runtime/config/loader.go`
- 共享配置 Fx provider：`common/runtime/configfx/config.go`
- 共享日志 Fx provider：`common/runtime/loggerfx/logger.go`
- 共享 datastore provider：`common/runtime/datastorefx/redis.go`、`common/runtime/datastorefx/postgres.go`
- 运行时资源名：`common/runtime/resources/resource_names.go`
- Atlas 配置：`user-service/atlas.hcl`
- 用户服务迁移目录：`user-service/migrations/`

## 4. Current Feature Areas

- 用户资料查询：`GET /api/v1/users/:id`
- 用户资料创建：`POST /api/v1/users`
- 用户列表分页查询：`GET /api/v1/users`
- 认证会话控制：登录、刷新、强制改密、退出当前设备、退出全部设备。
- HTTP 服务运行时：启动、运行、路由注册和优雅停止。
- 共享基础设施：配置、日志、Redis/PostgreSQL/Ent 运行时依赖。
- API 响应契约：统一成功/失败响应信封和应用错误映射。
- 数据库迁移：通过 Ent schema 和 Atlas 维护用户服务 SQL migration。

## 5. Development Commands

- 查看统一入口：`make help`。
- 构建用户服务二进制：`make build` 或 `make build-user-service`。
- 运行全部测试：`make test`。
- 运行用户服务：`make run-user-service`。
- 运行单模块测试：`make test-common` 或 `make test-user-service`。
- 生成 Ent 代码：`make generate`。
- 生成迁移：`make migrate-diff name=<name>`。
- 校验迁移：`make migrate-validate`。
- 执行迁移：`DATABASE_URL='<postgres-url>' make migrate-apply`。
- 生成 Swagger 文档：`make swagger-generate`。
- 格式化 Go 代码：`gofmt -w <files>`。
- 运行 lint：`make lint`，或按模块运行 `make lint-common`、`make lint-user-service`。

## 6. Change Workflow

1. 先阅读 `docs/ARCHITECTURE.md`，确认改动属于哪个模块、feature 和层。
2. 小改动直接实现；跨 feature、跨模块、目录结构或外部契约变更，应先在 issue、PR 描述或开发记录中写清目标、影响范围和验证方式。
3. 修改结构规则时，同步更新 `AGENTS.md` 和 `docs/ARCHITECTURE.md`。
4. 实现后运行与改动范围匹配的测试；跨模块变更分别在 `common/` 和 `user-service/` 验证。

## 7. Repository Rules

- 不要手写 `user-service/ent/` 下的生成代码；修改 Ent schema 后重新生成。
- 不要用运行时 `client.Schema.Create(ctx)` 表达 schema 变更；修改 Ent schema 后生成 Ent 代码和 Atlas SQL migration。
- 按 feature 组织服务内代码：用户资料放在 `internal/features/user`，认证会话放在 `internal/features/auth`。不要新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。
- 保持 `transport/http`、`app`、`domain`、`infra/*` 分层：HTTP 解析在 controller，业务编排在 app service，数据库或 Redis 访问在 infra adapter。
- 每个 feature 自己注册路由：`transport/http/routes.go` 暴露 `RegisterRoutes`，认证 feature 可拆分 `RegisterPublicRoutes` 和 `RegisterProtectedRoutes`；全局 router 只负责 `/api/v1`、认证中间件和 feature 路由总装。
- 每个 feature 自己提供 Fx module：`features/<feature>/module.go` 暴露 `Module` 并组装 feature 内部 service、controller 和 infra provider；`bootstrap.AppModule` 只保留共享运行时 provider、Gin engine、HTTP server 和路由 invoke。
- 基础设施目录统一使用 `infra/postgres/`、`infra/redis/` 等；不要使用 `store/` 作为目录名。
- 共享基础能力优先放在 `common/` 对应分类目录中；服务特定规则保留在服务模块内。
- HTTP API 应使用 `common/contract/response.Envelope` 格式返回。
- 配置通过 YAML 与 `AEGISCORE_` 环境变量覆盖加载，Redis/PostgreSQL 使用 `redis.<name>` 与 `postgres.<name>` 命名实例，避免硬编码运行时配置。
- `internal/shared` 默认禁止新增。只有当能力已被至少两个 feature 真实消费、边界稳定、且不能归入 `common` 时，才可以新增，并且必须在 `docs/ARCHITECTURE.md` 说明 owner、准入理由和禁止事项。
- Ports 由消费侧 feature 拥有：用户资料 service 消费的接口放在 `internal/features/user/app/ports.go`，认证 service 消费的凭据、token version 和 session 接口放在 `internal/features/auth/app/ports.go`。不要为了 adapter 方便在 infra 包或共享根包定义大接口。
- HTTP 请求 DTO 清洗、绑定后的输入规范化和简单字段解析放在对应 feature 的 `transport/http/validation.go`。这些函数不得导入 Ent、Redis、service、infra，或执行业务编排。
- Controller 必须把 transport DTO 映射为 command/query 后再调用 service，service 不接收 `api/*Request`。

| 层 | 可以依赖 | 禁止依赖 |
|---|---|---|
| `domain` | 标准库、稳定值对象 | Gin、Ent、Redis、config、response envelope |
| `app` | `domain`、消费侧端口接口、common 安全原语 | Gin、Ent、Redis、HTTP binder |
| `transport/http` | `api`、`app`、Gin、response envelope、feature-local validation | Ent、Redis、SQL |
| `infra/postgres` | Ent、SQL、app ports、domain | Gin、HTTP response |
| `infra/redis` | Redis client、app ports、domain | Gin、HTTP response |
| `module.go` | Fx、feature 内部包 | 业务逻辑 |

Adapter 可以做字段裁剪和模型转换，但不得承载复杂业务编排。禁止在 adapter 中实现登录状态机、密码校验、token 签发、跨 store 事务编排或 HTTP 错误映射。

Ent predicate 构造必须封装在 infra adapter 内，例如 `internal/features/user/infra/postgres/predicates.go`。`app/service.go` 不得导入 `github.com/aegiscore/user-service/ent/user`，也不得直接调用 Ent predicate。
