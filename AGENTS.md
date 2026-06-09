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
- `user-services/`：用户服务 HTTP 运行时，包含 Cobra 入口、Fx 组装、Gin 路由，以及按 capability 组织的 `internal/user` 与 `internal/auth` 包；同时包含 Ent schema、Atlas 配置、服务内 migration 和生成代码。
- `user-services/internal/user/`：用户资料 capability，包含 HTTP controller、应用 service、领域模型、command/query、消费侧 ports、`api/` DTO 和 `store/postgres/` Ent adapter。
- `user-services/internal/auth/`：认证会话 capability，包含 HTTP controller、应用 service、credential/token/session 组件、command/model/ports、Redis key builder、`api/` DTO 和 `store/redis/` session adapter。
- `user-services/internal/validators/`：请求 DTO 规范化和简单字段解析；必须保持纯净，不承载业务编排或 datastore 访问。
- `openspec/`：OPSX/OpenSpec 配置、主规格和后续 change artifacts。

## 3. Key Entry Points

- CLI 入口：`user-services/cmd/main.go`
- 服务组装：`user-services/internal/bootstrap/app.go`
- HTTP 路由：`user-services/internal/router/router.go`
- 用户 controller：`user-services/internal/user/controller.go`
- 用户 service：`user-services/internal/user/service.go`
- 用户 PostgreSQL store：`user-services/internal/user/store/postgres/user_store.go`
- 认证 controller：`user-services/internal/auth/controller.go`
- 认证 service：`user-services/internal/auth/service.go`
- 认证 session store：`user-services/internal/auth/store/redis/session_store.go`
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
- 按 capability 组织服务内代码：用户资料放在 `internal/user`，认证会话放在 `internal/auth`。不要重新新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。
- 保持 controller/service/store 分层：HTTP 解析在 controller，业务编排在 service，数据库或 Redis 访问在 `store/*` adapter。
- 共享基础能力优先放在 `common/` 对应能力分类目录中，避免在服务模块中重复实现中间件、响应信封或基础设施初始化；服务特定规则保留在服务模块内。
- HTTP API 应使用 `common/contract/response.Envelope` 格式返回。
- 配置通过 YAML 与 `AEGISCORE_` 环境变量覆盖加载，Redis/PostgreSQL 使用 `redis.<name>` 与 `postgres.<name>` 命名实例，避免硬编码运行时配置。
- `internal/shared` 默认禁止新增。只有当能力已被至少两个 capability 真实消费、边界稳定、且不能归入 `common` 时，才可在 proposal/design 中说明 owner、准入理由和禁止事项后新增。
- Ports 由消费侧 capability 拥有：用户资料 service 消费的接口放在 `internal/user/ports.go`，认证 service 消费的凭据、token version 和 session 接口放在 `internal/auth/ports.go`。不要为了 adapter 方便在 store 包或共享根包定义大接口。
- `internal/validators` 只能依赖请求 DTO、共享校验/响应原语和标准库；不得导入 `gin`、Ent、Redis、service、store，或执行业务编排。
- Controller 必须把 transport DTO 映射为 command/query 后再调用 service，service 不接收 `api/*Request`。

  ```go
  req := userapi.CreateUserRequest{}
  if !ginvalidation.BindOrAbort(ctl.validator, c, &req, ginvalidation.JSONBinder) {
      return
  }
  if err := validators.NormalizeCreateUser(&req); err != nil {
      response.Fail(c, err)
      return
  }
  created, err := ctl.userService.CreateUser(c.Request.Context(), user.CreateUserCommand{
      Nickname: req.Nickname,
      Username: req.Username,
      Password: req.Password,
      Status:   toCommandStatus(req.Status),
  })
  ```

- Adapter 可以做字段裁剪和模型转换，但不得承载复杂业务编排。允许示例：

  ```go
  type AuthUserAdapter struct {
      store auth.UserCredentialStore
  }

  func (a AuthUserAdapter) GetByUsername(ctx context.Context, username string) (*auth.UserCredential, error) {
      credential, err := a.store.GetByUsername(ctx, strings.TrimSpace(username))
      if err != nil {
          return nil, err
      }
      return &auth.UserCredential{
          UserID:       credential.UserID,
          Username:     credential.Username,
          PasswordHash: credential.PasswordHash,
          Status:       credential.Status,
          TokenVersion: credential.TokenVersion,
      }, nil
  }
  ```

  禁止在 `adapter.go` 中实现登录状态机、密码校验、token 签发、跨 store 事务编排或 HTTP 错误映射。
- Ent predicate 构造必须封装在 store 内，例如 `internal/user/store/postgres/predicates.go`：

  ```go
  func buildListPredicates(input user.ListUsersInput) []predicate.User {
      predicates := []predicate.User{entuser.DeletedAtIsNil()}
      if input.Status != nil {
          predicates = append(predicates, entuser.StatusEQ(int64(*input.Status)))
      }
      return predicates
  }
  ```

  反例：`internal/user/service.go` 不得导入 `github.com/aegiscore/user-services/ent/user`，也不得直接调用 `user.StatusEQ(...)` 或其他 Ent predicate。
