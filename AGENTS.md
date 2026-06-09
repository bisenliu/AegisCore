# AegisCore Agent Guide

本文件为 AI 代理提供仓库导航。修改代码前先确认改动所属 capability，并优先阅读对应文档与主规格。

## 1. Quick Start

- 架构文档：`docs/ARCHITECTURE.md`
- 开发说明：`docs/DEVELOPMENT.md`
- 产品上下文：`docs/PRODUCT.md`
- 测试说明：`docs/TESTING.md`
- 能力地图：`docs/opsx/CAPABILITY_MAP.md`
- OPSX 工作流：`docs/opsx/CHANGE_WORKFLOW.md`

## 2. Repository Shape

- `go.work`：Go workspace，包含 `common` 和 `user-services` 两个模块。
- `common/`：按 `contract`、`runtime`、`http`、`security`、`validation` 分类组织跨服务稳定契约和基础能力，不作为服务特定 helper 的兜底目录。
- `user-services/`：用户服务 HTTP 运行时，包含 Cobra 入口、Fx 组装、Gin 路由，以及按 capability 组织的 `internal/features/user` 与 `internal/features/auth` 包；同时包含 Ent schema、Atlas 配置、服务内 migration 和生成代码。
- `user-services/internal/features/user/`：用户资料 capability，按 `api/` HTTP DTO 和 Swagger 文档模型、`app/` service/command/query/ports/mapper、`domain/` 领域实体/枚举/错误、`transport/http/` Gin controller/route/validation、`infra/postgres/` Ent adapter 和 `module.go` 分层。
- `user-services/internal/features/auth/`：认证会话 capability，按 `api/` HTTP DTO、`app/` service/credential/token/session/commands/ports、`domain/` 会话/凭据模型/Redis key 语义/领域错误、`transport/http/` Gin controller/route/validation、`infra/postgres/` credential/token-version adapter、`infra/redis/` session adapter 和 `module.go` 分层。
- `openspec/`：OPSX/OpenSpec 配置、主规格和后续 change artifacts。

## 3. Key Entry Points

- CLI 入口：`user-services/cmd/main.go`
- 服务组装：`user-services/internal/bootstrap/app.go`
- HTTP 路由：`user-services/internal/router/router.go`
- 用户 feature module：`user-services/internal/features/user/module.go`
- 用户 controller：`user-services/internal/features/user/transport/http/controller.go`
- 用户 service：`user-services/internal/features/user/app/service.go`
- 用户 PostgreSQL infra adapter：`user-services/internal/features/user/infra/postgres/user_store.go`
- 认证 feature module：`user-services/internal/features/auth/module.go`
- 认证 controller：`user-services/internal/features/auth/transport/http/controller.go`
- 认证 service：`user-services/internal/features/auth/app/service.go`
- 认证 PostgreSQL credential infra adapter：`user-services/internal/features/auth/infra/postgres/credential_store.go`
- 认证 session infra adapter：`user-services/internal/features/auth/infra/redis/session_store.go`
- 共享配置加载：`common/runtime/config/loader.go`
- 共享配置 Fx provider：`common/runtime/configfx/config.go`
- 共享日志 Fx provider：`common/runtime/loggerfx/logger.go`
- 共享 datastore provider：`common/runtime/datastorefx/redis.go`、`common/runtime/datastorefx/postgres.go`
- 运行时资源名：`common/runtime/resources/resource_names.go`
- Atlas 迁移配置：`user-services/atlas.hcl`
- 用户服务迁移目录：`user-services/migrations/`
- 迁移脚本：`user-services/scripts/migrate-diff.sh`、`user-services/scripts/migrate-validate.sh`、`user-services/scripts/migrate-apply.sh`

## 4. Core Capabilities

- `user-profile-query`：通过 `GET /api/v1/users/:id` 查询用户资料。
- `user-profile-create`：通过 `POST /api/v1/users` 创建用户资料。
- `user-list-query`：通过 `GET /api/v1/users` 分页查询用户资料。
- `user-session-control`：支持登录、刷新、强制改密、退出当前设备和退出全部设备。
- `http-service-runtime`：启动、运行和优雅停止用户服务 HTTP server。
- `shared-infrastructure`：加载配置，提供 Zap 日志，并支持服务侧声明具名 Redis/PostgreSQL/Ent 运行时依赖。
- `api-response-contract`：统一成功/失败响应信封和应用错误映射。
- `common-module-organization`：约束 common 模块目录分类和共享能力准入边界。
- `database-schema-migrations`：通过 Ent schema 和 Atlas 维护用户服务 SQL migration。
- `go-toolchain-baseline`：统一 `go.work`、`common/go.mod` 和 `user-services/go.mod` 的 Go 1.26 工具链基线。

详见 `docs/opsx/CAPABILITY_MAP.md` 与 `openspec/specs/*/spec.md`。

## 5. Development Commands

- 运行全部测试：分别在 `common/` 和 `user-services/` 执行 `go test ./...`。
- 运行用户服务：`go run ./user-services/cmd serve --config ./user-services/configs/config.yaml`。
- 生成 Ent 代码：在 `user-services` 模块中运行 `go generate ./ent`。
- 生成迁移：在 `user-services/` 执行 `./scripts/migrate-diff.sh <name>`。
- 校验迁移：在 `user-services/` 执行 `./scripts/migrate-validate.sh`。
- 格式化 Go 代码：`gofmt -w <files>`。

## 6. Change Workflow

1. 先阅读 `docs/opsx/CAPABILITY_MAP.md`，定位相关 capability。
2. 如需求不清，先用 `/opsx:explore` 澄清问题和方案。
3. 用 `/opsx:propose <change-name>` 创建 proposal、design、tasks。
4. 准备实现时使用 `/opsx:apply <change-name>`。
5. 实现后验证测试，再用 `/opsx:archive <change-name>` 归档已完成 change。

## 7. Repository Rules

- 不要手写 `user-services/ent/` 下的生成代码；修改 Ent schema 后重新生成。
- 不要用运行时 `client.Schema.Create(ctx)` 表达 schema 变更；修改 Ent schema 后生成 Ent 代码和 Atlas SQL migration。
- 按 capability 组织服务内代码：用户资料放在 `internal/features/user`，认证会话放在 `internal/features/auth`，并在 feature 内使用 `api/app/domain/transport/http/infra/*` 分层。不要重新新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。
- 保持 transport/app/infra 分层：HTTP 解析在 `transport/http` controller，业务编排在 `app` service，数据库或 Redis 访问在 `infra/*` adapter。
- 每个 feature 自己注册路由：`transport/http/routes.go` 暴露 `RegisterRoutes`，认证 feature 可拆分 `RegisterPublicRoutes` 和 `RegisterProtectedRoutes`；全局 router 只负责 `/api/v1`、认证中间件和 feature 路由总装。
- 每个 feature 自己提供 Fx module：`features/<feature>/module.go` 暴露 `Module` 并组装 feature 内部 service、controller 和 infra provider；`bootstrap.AppModule` 只保留共享运行时 provider、Gin engine、HTTP server 和路由 invoke。
- 不要使用 `store/` 作为基础设施目录名，统一使用 `infra/postgres/`、`infra/redis/` 等，以避免和未来门店业务概念冲突。
- 共享基础能力优先放在 `common/` 对应能力分类目录中，避免在服务模块中重复实现中间件、响应信封或基础设施初始化；服务特定规则保留在服务模块内。
- HTTP API 应使用 `common/contract/response.Envelope` 格式返回。
- 配置通过 YAML 与 `AEGISCORE_` 环境变量覆盖加载，Redis/PostgreSQL 使用 `redis.<name>` 与 `postgres.<name>` 命名实例，避免硬编码运行时配置。
- `internal/shared` 默认禁止新增。只有当能力已被至少两个 capability 真实消费、边界稳定、且不能归入 `common` 时，才可在 proposal/design 中说明 owner、准入理由和禁止事项后新增。
- Ports 由消费侧 capability 拥有：用户资料 service 消费的接口放在 `internal/features/user/app/ports.go`，认证 service 消费的凭据、token version 和 session 接口放在 `internal/features/auth/app/ports.go`。不要为了 adapter 方便在 infra 包或共享根包定义大接口。
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

  ```go
  req := userapi.CreateUserRequest{}
  if !ginvalidation.BindOrAbort(ctl.validator, c, &req, ginvalidation.JSONBinder) {
      return
  }
  if err := NormalizeCreateUser(&req); err != nil {
      response.Fail(c, err)
      return
  }
  created, err := ctl.userService.CreateUser(c.Request.Context(), userapp.CreateUserCommand{
      Nickname: req.Nickname,
      Username: req.Username,
      Password: req.Password,
      Status:   toCommandStatus(req.Status),
  })
  ```

- Adapter 可以做字段裁剪和模型转换，但不得承载复杂业务编排。允许示例：

  ```go
  type AuthUserAdapter struct {
      store authapp.UserCredentialStore
  }

  func (a AuthUserAdapter) GetByUsername(ctx context.Context, username string) (*authdomain.UserCredential, error) {
      credential, err := a.store.GetByUsername(ctx, strings.TrimSpace(username))
      if err != nil {
          return nil, err
      }
      return &authdomain.UserCredential{
          UserID:       credential.UserID,
          Username:     credential.Username,
          PasswordHash: credential.PasswordHash,
          Status:       credential.Status,
          TokenVersion: credential.TokenVersion,
      }, nil
  }
  ```

  禁止在 `adapter.go` 中实现登录状态机、密码校验、token 签发、跨 store 事务编排或 HTTP 错误映射。
- Ent predicate 构造必须封装在 infra adapter 内，例如 `internal/features/user/infra/postgres/predicates.go`：

  ```go
  func buildListPredicates(input userapp.ListUsersInput) []predicate.User {
      predicates := []predicate.User{entuser.DeletedAtIsNil()}
      if input.Status != nil {
          predicates = append(predicates, entuser.StatusEQ(int64(*input.Status)))
      }
      return predicates
  }
  ```

  反例：`internal/features/user/app/service.go` 不得导入 `github.com/aegiscore/user-services/ent/user`，也不得直接调用 `user.StatusEQ(...)` 或其他 Ent predicate。
