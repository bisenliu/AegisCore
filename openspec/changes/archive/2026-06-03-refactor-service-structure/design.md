## Context

AegisCore 当前是 Go workspace，`common` 承载共享响应契约、配置、日志和基础设施，`user-services` 承载用户服务 HTTP runtime、业务分层、Ent schema 和 Atlas migration。用户反馈集中在三个维护性问题：Ent schema 需要按领域分类以承载未来实体扩展，`common/response/response.go` 同时承担响应、分页和错误 helper，`user-services/internal/bootstrap/bootstrap.go` 同时承担 Fx app、Gin、JWT、路由、HTTP server 和资源装配职责。

本变更属于跨 capability 的内部结构重构，涉及 `database-schema-migrations`、`api-response-contract`、`http-service-runtime` 和 `shared-infrastructure`。实现必须保持 controller/service/repository 分层不变，保持 HTTP API、响应信封、错误码、配置键、Redis/PostgreSQL 命名实例、Ent schema 字段和 Atlas migration 历史不变。

## Goals / Non-Goals

**Goals:**

- 将 `common/response` 拆分为职责明确的文件边界：响应信封和 Gin helper、分页类型和计算、消息常量和错误相关 helper 保持可定位。
- 将 `user-services/internal/bootstrap` 拆分为职责明确的启动装配文件：Fx app/module、Gin engine、JWT provider、路由注册、HTTP server lifecycle、datastore/Ent wiring 分别维护。
- 为 `user-services/ent/schema/` 引入实际领域分类，将当前 `User` schema 移入用户领域分类，并保持 Ent 生成和 Atlas migration source 可用。
- 保持现有导出符号、JSON 响应、Fx 依赖图、运行时依赖初始化和测试入口行为等价。

**Non-Goals:**

- 不新增 `organization`、`role`、`permission` 等实体。
- 不修改 Ent `User` schema 字段、索引、表结构或生成新的 Atlas SQL migration。
- 不重命名 Go module、CLI、服务名、配置路径、HTTP 路由或外部响应字段。
- 不引入新的基础设施目录大规模迁移，例如将 `common/infrastructure` 整体拆为子包。

## Decisions

1. 响应包按文件拆分而不是按子包拆分。

   选择在 `common/response` 包内拆为 `envelope.go`、`pagination.go`、`messages.go` 和 `helpers.go` 等聚焦文件。这样可以降低单文件职责混杂，同时保持 `response.OK`、`response.NewPagination`、`response.BadRequest` 等现有调用方 API 不变。备选方案是新增 `common/response/pagination` 或 `common/response/errors` 子包，但会扩大 import 变更并破坏当前简洁调用方式。

2. Bootstrap 按组合根职责拆文件而不拆包。

   选择保留 `user-services/internal/bootstrap` 包，将当前 `bootstrap.go` 中的函数移动到聚焦文件，例如 `app.go` 或 `fx.go` 承载 `NewApp` 和 Fx module，`gin.go` 承载 Gin engine，`auth.go` 承载认证 provider，`routes.go` 承载路由注册，`server.go` 承载 HTTP server lifecycle。拆分后不保留 `bootstrap/bootstrap.go` 这种包名与文件名重复且职责泛化的聚合文件。这样 Fx provider 仍处于同一组合根，避免为内部启动装配引入额外包依赖。备选方案是拆成 `bootstrap/http`、`bootstrap/auth` 等子包，但会让 Fx wiring 的跨文件依赖变得更分散。

3. Ent schema 本次实际引入领域分类，并通过根 schema 包保持生成入口稳定。

   选择在 `user-services/ent/schema/` 下增加用户领域分类目录，例如 `schema/user/`，并通过根 `schema` 包中的聚合/转发文件继续向 Ent codegen 暴露 `User` schema。这样当前实体先进入分类布局，同时尽量保持 `go generate ./ent` 与 Atlas `ent://ent/schema` 入口稳定。备选方案是只写文档规划后续迁移，但不能解决当前目录缺乏分类的问题；另一个备选方案是直接把 codegen source 改为子包入口，但未来多领域 schema 会需要多个 source 聚合，扩展性较差。

4. 使用测试和静态验证保障行为等价。

   实现完成后应运行 `go test ./...` 分别覆盖 `common/` 与 `user-services/`，并必须在 `user-services` 中运行 `go generate ./ent` 验证 Ent 分类布局可生成。由于 `User` 字段和索引不变，Atlas diff 不应产生数据库结构 migration；仍应运行 migration inspect 或 validate 类命令确认 schema source 可读取。

## Risks / Trade-offs

- [Risk] 文件拆分可能遗漏导出符号或 import，导致编译失败。→ 通过 `go test ./...` 验证两个 Go 模块。
- [Risk] Bootstrap 拆分时改变 Fx provider 顺序或生命周期 hook。→ 保持 provider 函数签名和 `UserServiceModule` 的 provider/invoke 集合等价，只移动代码位置。
- [Risk] Ent schema 子目录分类可能不被现有 Ent/Atlas source 自动加载。→ 通过根 `schema` 包聚合实体类型，并用 `go generate ./ent` 与 Atlas inspect/validate 验证入口可用。
- [Risk] 响应包拆分可能被误解为响应契约变更。→ 明确要求 JSON 字段、业务码、消息常量和 helper 行为不变。

## Migration Plan

- 第一步拆分 `common/response` 文件，保持包名和导出 API 不变。
- 第二步拆分 `user-services/internal/bootstrap` 文件，保持 `NewApp`、`UserServiceModule`、provider 和 invoke 行为不变。
- 第三步将 `User` Ent schema 纳入用户领域分类目录，并保留根 `schema` 包作为 Ent codegen 与 Atlas source 的稳定入口；不得改变字段、索引或注释语义。
- 第四步运行 `go generate ./ent`、Atlas schema source 验证和 Go 测试。若 Atlas diff 因字段或索引变化产生 SQL，应视为实现错误并回退字段级变更；本变更不应新增 migration。
- 回滚策略为恢复文件组织到变更前布局；由于不改变外部契约和数据库结构，回滚不需要数据迁移。

## Open Questions

- 无阻塞问题。实现时需要以 `go generate ./ent` 和 Atlas schema source 验证最终分类方式。
