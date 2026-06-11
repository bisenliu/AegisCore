# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和本 change 的 `proposal.md`、`design.md`，确认 provider split 不改变 feature 分层、HTTP 行为或配置契约。
- [x] 新增 `user-service/internal/providers/module.go`，提供 `providers.Module` 统一组装服务级 providers 和 `RegisterRoutes` invoke。
- [x] 将 `user-service/internal/bootstrap/gin.go` 迁移到 `user-service/internal/providers/gin.go`，包名改为 `providers`，保留 Gin mode、trusted proxies 和 middleware 顺序。
- [x] 将 `user-service/internal/bootstrap/routes.go` 迁移到 `user-service/internal/providers/routes.go`，包名改为 `providers`，保留 route params、optional token version validator 和 router 调用语义。
- [x] 将 `user-service/internal/bootstrap/auth.go` 迁移到 `user-service/internal/providers/auth.go`，包名改为 `providers`，保留 JWT service 构造语义。
- [x] 将 `user-service/internal/bootstrap/postgres.go` 迁移到 `user-service/internal/providers/postgres.go`，包名改为 `providers`，保留 `user_db`、`common_db` named PostgreSQL outputs。
- [x] 将 `user-service/internal/bootstrap/redis.go` 迁移到 `user-service/internal/providers/redis.go`，包名改为 `providers`，保留 `cache_redis` named Redis output。
- [x] 将 `user-service/internal/bootstrap/ent.go` 迁移到 `user-service/internal/providers/ent.go`，包名改为 `providers`，保留 non-closing Ent driver 和 Ent close lifecycle hook。
- [x] 更新 `user-service/internal/bootstrap/app.go`，导入 `internal/providers` 并通过 `providers.Module` 或显式 provider 引用组装服务级基础设施。
- [x] 确保 `bootstrap.AppModule` 仍导入 `commontz.Module`、`validation.Module`、`features/auth.Module`、`features/user.Module`，并继续提供 `NewHTTPServer`。
- [x] 确保 `bootstrap.AppModule` 仍 invoke `func(*http.Server) {}`，使 HTTP server lifecycle hook 被实例化注册。
- [x] 删除迁移后的 bootstrap provider 源文件，确保 `bootstrap` 不保留 forwarding shim 或重复实现。
- [x] 运行 `gofmt -w` 格式化所有改动的 Go 文件。

## Tests

- [x] 将 Ent provider 相关测试从 `user-service/internal/bootstrap` 迁移到 `user-service/internal/providers`，更新 package、import 和未导出 helper 访问方式。
- [x] 将 PostgreSQL provider 相关测试迁移到 `user-service/internal/providers`，确认 named output 和资源名覆盖不变。
- [x] 将 Redis provider 相关测试迁移到 `user-service/internal/providers`，确认 `cache_redis` 行为不变。
- [x] 将 Gin provider 和 route registration 相关测试迁移到 `user-service/internal/providers`。
- [x] 保留 HTTP server 生命周期测试在 `user-service/internal/bootstrap`，只更新必要 import 或 package references。
- [x] 检查是否需要新增 `providers.Module` smoke test，验证 module 能和 feature modules、runtime config/logger 一起被 Fx 装配。
- [x] 确认测试没有通过导入 `bootstrap` 来访问已迁移的 provider。

## Documentation

- [x] 更新 `AGENTS.md` 的 Repository Shape，说明 `user-service/internal/providers` 承载服务级 provider。
- [x] 更新 `AGENTS.md` 的 Key Entry Points，加入 provider module、Gin/routes/auth/PostgreSQL/Redis/Ent provider 文件，并调整 bootstrap 入口说明。
- [x] 更新 `AGENTS.md` 的 Repository Rules，明确 bootstrap 只负责 Fx app/module 总装和 HTTP server lifecycle，service-level provider 放在 `internal/providers`。
- [x] 更新 `docs/ARCHITECTURE.md` Runtime Flow，使 service provider 由 `internal/providers` 提供，bootstrap 只负责顶层组装和 server lifecycle。
- [x] 更新 `docs/ARCHITECTURE.md` HTTP Request Flow 中 middleware 和 route assembly 的代码位置。
- [x] 更新 `docs/ARCHITECTURE.md` Infrastructure 中 Ent clients 和 datastore named provider 的服务侧位置。
- [x] 更新 `docs/DEVELOPMENT.md` Coding Conventions，说明服务级 Fx provider 放在 `user-service/internal/providers`。
- [x] 确认文档仍声明不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `rg -n "func NewGinEngine|func RegisterRoutes|func NewJWTService|func ProvidePostgresPools|func ProvideRedisClients|func ProvideEntClients|type Named(Postgres|Redis|Ent)" user-service/internal/bootstrap`，确认 bootstrap 中没有迁移后的 provider 实现。
- [x] 运行 `rg -n "package providers" user-service/internal/providers`，确认 provider 文件都在新包下。
- [x] 运行 `rg -n "internal/bootstrap|bootstrap\\." user-service/internal/providers`，确认 providers 不依赖 bootstrap。
- [x] 运行 `rg -n "providers\\.Module|internal/providers" user-service/internal docs AGENTS.md`，确认新 package 被 AppModule 和文档引用。
- [x] 在 `common/` 运行 `go test ./...`。
- [x] 在 `user-service/` 运行 `go test ./...`。
- [x] 检查 `git diff -- user-service/internal AGENTS.md docs`，确认没有 HTTP API、配置 key、Ent schema、migration 或 generated code 变更。

## Review Notes

- [x] 确认没有移动 `features/user` 或 `features/auth` 内部业务代码。
- [x] 确认没有把服务特定 provider 放入 `common`。
- [x] 确认 `router` 包没有新增 Fx provider 或反向依赖 `providers`。
- [x] 确认 PostgreSQL 只初始化 `user_db` 和 `common_db`，Redis 只初始化 `cache_redis`。
- [x] 确认 Ent client close 不会关闭 datastore-owned SQL pools。
- [x] 确认没有新增 tracing、metrics、transaction、eventbus 的真实实现。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
