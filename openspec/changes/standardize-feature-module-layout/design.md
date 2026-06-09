## Context

用户服务当前已经完成第一轮能力聚合：用户资料位于 `user-services/internal/features/user`，认证会话位于 `user-services/internal/features/auth`，每个 feature 内已有 `api`、`app`、`domain`、`store` 分层。但现有结构仍有四个长期扩展问题：

- `app/` 同时承载 HTTP controller 和业务用例，导致 Gin、响应输出、DTO 绑定与 use case/service 边界混在同一个包。
- `store/` 作为基础设施 adapter 命名会与后续门店业务 capability 冲突，不利于 `features/store` 这类业务模块出现。
- `internal/validators` 是全局服务包，但当前 user/auth 校验函数本质上处理 feature HTTP DTO 的清洗、解析和 transport-safe 规则。
- `bootstrap` 和 `router` 仍直接知道所有 feature 的 controller、service 和 infra provider 细节，后续接口数量增长后组合根会持续变胖。

本变更只调整 `user-services/internal` 的内部结构和 OpenSpec/文档描述，不改变 `common` 模块、HTTP 外部契约、配置、Redis/PostgreSQL 命名实例、Ent schema 或 Atlas migration。目标结构为：

```text
user-services/internal/features/
  auth/
    api/
    app/
    domain/
    transport/http/
    infra/postgres/
    infra/redis/
    module.go
  user/
    api/
    app/
    domain/
    transport/http/
    infra/postgres/
    module.go
```

## Goals / Non-Goals

**Goals:**

- 将 user/auth controller、handler tests、route registration 和 HTTP-local validation 迁移到 `features/<feature>/transport/http`。
- 将 user/auth PostgreSQL、Redis adapter 从 `store/*` 迁移到 `infra/*`，保持端口由 `app/ports.go` 拥有。
- 保持 `app/` 只表达业务用例执行：commands/queries、ports、service、credential/token/session 组件和 mapper，不依赖 Gin、HTTP binder 或 response writer。
- 让每个 feature 暴露自己的 `Module`，由 `bootstrap.AppModule` 引入 feature modules，而不是在 bootstrap 中逐个装配 feature 内部实现。
- 让每个 feature 暴露自己的 HTTP route registration；全局 HTTP runtime 只负责系统路由、Swagger、API version 分组、认证中间件分组和 feature route 总装。
- 更新 tests、Swagger 注解、docs、AGENTS.md、capability map 和 OpenSpec 主规格路径引用。
- 保持现有 HTTP API、响应信封、错误码、认证边界、配置 key、Redis key、数据库 schema、Ent 生成代码、Atlas migration 和 Go module path 不变。

**Non-Goals:**

- 不新增用户、认证、授权、支付、门店、团队或审计 API。
- 不改变登录、刷新、强制改密、登出、用户创建、用户查询或用户列表的业务语义。
- 不修改 `common` 共享能力边界，不把用户服务特定校验规则上移到 `common/validation`。
- 不新增 `internal/shared` 或横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api`、`internal/domain` 包。
- 不修改 Ent schema，不运行 Ent codegen，不生成 Atlas migration。
- 不引入新的外部依赖。

## Decisions

### Decision: HTTP transport 独立为 feature-local adapter

将 `features/user/app/controller.go` 移至 `features/user/transport/http/controller.go`，将 `features/auth/app/controller.go` 移至 `features/auth/transport/http/controller.go`。HTTP 包可以依赖 `api` DTO、`app` command/query/service、Gin、`common/http/ginvalidation` 和 `common/contract/response`；`app` 包不得反向依赖 HTTP transport。

理由：controller 是入口 adapter，不是业务用例本体。把 Gin binding、response envelope 输出和 route handler 测试移出 `app` 后，`app` 能稳定表达“用例如何执行”，未来 CLI、内部 RPC 或 job 调用也不会被 HTTP DTO 绑住。

备选方案是继续把 controller 放在 `app`，迁移成本最低，但会让 `app` 长期混入 transport 依赖，后续 feature 增长时难以审查边界。

### Decision: `store` 改名为 `infra`

将 `features/user/store/postgres` 改为 `features/user/infra/postgres`，将 `features/auth/store/postgres` 和 `features/auth/store/redis` 改为 `features/auth/infra/postgres`、`features/auth/infra/redis`。

理由：这些包是 PostgreSQL/Redis 基础设施 adapter，而不是业务“门店 store”。`infra` 能清楚表达它们可以依赖 Ent、SQL、Redis client 和 app ports/domain，但不能依赖 Gin 或 HTTP response。

备选方案是保留 `store` 并在文档中区分 repository/store 与门店业务。该方案会给未来 `features/store` 业务模块留下持续歧义。

### Decision: HTTP validation 下沉到 `transport/http`

将 `internal/validators/user.go` 和 `internal/validators/auth.go` 的 HTTP DTO 清洗、基础解析、transport-safe 规则迁移到对应 feature 的 `transport/http/validation.go`。

理由：这些函数直接处理 feature API request DTO，属于 HTTP 入口边界。下沉后 validator 与 controller 同包或同层，可以减少全局包对 feature DTO 的反向聚合。涉及持久化状态、session 状态、用户名唯一性或 token version 的检查仍必须保留在 app service 编排中。

备选方案是保留全局 `internal/validators`。该方案历史兼容性更好，但会让每个新增 feature 都把入口清洗函数继续塞进全局包，最终变成第二个无边界 shared。

### Decision: feature 自己注册路由

用户 feature 的 `transport/http/routes.go` 提供 `RegisterRoutes(group *gin.RouterGroup, controller *Controller)`。认证 feature 提供 `RegisterPublicRoutes` 和 `RegisterProtectedRoutes`，分别注册登录/刷新/受限改密以及登出/登出全部设备。全局 router/bootstrap 只创建 `/api/v1`、公开分组、受保护分组，并挂载 auth middleware 后调用 feature route 函数。

理由：路由归属随 feature 增长而保持局部可见，未来新增 finance、payment、store、team 时不会把一个全局 `internal/router` 撑成所有业务 endpoint 的清单。

备选方案是保留全局 router 的 `auth.go`、`users.go` 分文件。它能短期控制统一路由视图，但业务 endpoint 越多，全局 router 越容易承担 feature ownership。

### Decision: feature 暴露 Fx module

在 `features/auth/module.go` 和 `features/user/module.go` 中定义 `var Module = fx.Module(...)`，分别提供 feature 内部 infra adapter、app service 和 HTTP controller。`bootstrap.AppModule` 继续负责共享配置、Zap logger、timezone、validation、PostgreSQL/Redis/Ent runtime、Gin engine、HTTP server 和 route 总装，但不再逐个列出 feature 内部 provider。

理由：bootstrap 是进程组合根，不应该了解每个 feature 内部构造函数的全部细节。feature module 能把 owner 边界固化到代码结构中，同时仍让运行时资源和 named injection 留在服务组合根控制。

备选方案是继续在 bootstrap 中 `fx.Provide` 所有 feature provider。它最直观，但随着接口和 adapter 增多会降低组合根可维护性。

### Decision: 依赖矩阵作为实现审查规则

最终依赖边界为：

| 层 | 可以依赖 | 禁止依赖 |
|---|---|---|
| `domain` | 标准库、稳定值对象 | Gin、Ent、Redis、config、response envelope |
| `app` | `domain`、消费侧端口接口、common 安全原语 | Gin、Ent、Redis、HTTP binder |
| `transport/http` | `api`、`app`、HTTP validation、Gin、response envelope | Ent、Redis、SQL |
| `infra/postgres` | Ent、SQL、app ports、domain | Gin、HTTP response |
| `infra/redis` | Redis client、app ports、domain | Gin、HTTP response |
| `module.go` | Fx、feature 内部包 | 业务逻辑 |

理由：这个矩阵能让实现和 review 不只是“文件移动”，而是把依赖方向固定下来。

## Risks / Trade-offs

- 包路径迁移影响面较大 -> 先完成机械移动和 import rewiring，再收紧依赖边界；使用 `rg "features/.*/app/controller|features/.*/store|internal/validators"` 和 `go test ./...` 兜底。
- Fx module 拆分可能暴露 named injection 或接口 provider 缺口 -> 保持 runtime resources 仍由 bootstrap 声明，feature module 只消费已存在的 Ent/Redis/auth config/logging provider。
- controller 包迁移可能影响 Swagger 注解扫描和生成 schema 名称 -> 更新注解 import，必要时重新生成 Swagger docs 并核对 OpenAPI 路径和响应字段未漂移。
- validation 下沉可能造成 HTTP 与非 HTTP 调用的输入归一化不一致 -> 只下沉 transport-safe DTO 清洗；影响 repository 输入、分页默认值或用例执行语义的归一化继续由 app service 统一处理。
- feature module 增加包间引用需要避免循环依赖 -> `module.go` 位于 feature 根包，只依赖同 feature 的 `app`、`transport/http`、`infra/*`；`app`、`domain` 和 `infra/*` 不得依赖 feature 根包。
- 文档和主规格中旧路径较多 -> 将路径更新作为 tasks 的独立验证项，归档前用 `rg` 检查非 archive 的 stale references。

## Migration Plan

1. 创建 `features/{user,auth}/transport/http`、`features/{user,auth}/infra/*` 和 feature 根 `module.go`。
2. 移动 user/auth controller、controller tests、route registration 和 HTTP validation 到 `transport/http`，更新 package/import 和 Swagger 注解。
3. 移动 `store/postgres`、`store/redis` 到 `infra/postgres`、`infra/redis`，保持构造函数、端口实现和 Ent/Redis 映射行为等价。
4. 在 user/auth feature 根 package 添加 `Module`，把 feature 内 provider 从 bootstrap 移入 module。
5. 调整全局 route 总装：系统路由和 Swagger 仍在服务级 runtime，业务路由调用 feature `transport/http` 注册函数。
6. 删除或清空 `internal/validators` 中的 user/auth HTTP validation 逻辑；如果目录没有剩余职责则移除服务级 validators 包引用。
7. 更新 AGENTS.md、docs、capability map、OpenSpec 主规格和测试路径引用。
8. 运行 `gofmt`，在 `user-services/` 执行 `go test ./...`；如果 `common/` 未改动，可记录无需运行 common 测试，但最终回归建议仍执行 `common/` 的 `go test ./...`。

回滚策略：本变更不改变外部协议、数据库、Redis 或配置。若迁移失败，可按目录移动和 import 更新回退，不需要数据库回滚、Redis 数据清理或配置迁移。

## Open Questions

无阻塞问题。实现时若发现某个函数不属于 HTTP transport 但也不是 app service 用例，应优先按依赖矩阵归入最近层；只有确实被多个 feature 稳定消费且无法通过 port/DI 表达时，才可另行提出 `internal/shared` 例外。
