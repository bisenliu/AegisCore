## 1. Baseline URL Catalog

- [x] 1.1 梳理当前 `user-service/internal/router/router.go` 和各 feature `transport/http/routes.go`，确认所有进入 `permissionhttp.Authorize` 的业务路由清单。
- [x] 1.2 在 `permission/application/rbacbaseline/catalog.go` 为权限目录、角色管理、用户角色绑定和角色权限绑定接口补充稳定 `permission_id` 常量。
- [x] 1.3 补齐 `DefaultPermissions()`，确保 method/path template 与 Gin 注册完全一致。
- [x] 1.4 更新 `DefaultRolePermissions()`，确保超级管理员默认绑定所有系统权限。

## 2. Interface Documentation

- [x] 2.1 检查 user、permission、role controller 的 Swagger 注解，确认 `@Router` 路径、HTTP method、入参和返回结构与真实路由一致。
- [x] 2.2 如发现缺失或不一致，更新对应 controller 注解，不改变真实 HTTP 行为。
- [x] 2.3 必要时更新 `docs/DEVELOPMENT.md` 或相关变更说明，说明新增受保护路由时必须同步更新 RBAC baseline catalog、执行 seed 并检查 route diff。

## 3. Tests

- [x] 3.1 增强 `permission/application/rbacbaseline/catalog_test.go`，校验权限 ID 唯一、route identity 唯一、默认绑定引用完整。
- [x] 3.2 新增或增强测试，断言 `DefaultPermissions()` 覆盖当前所有可授权业务路由，并排除 auth/health/swagger/OPTIONS。
- [x] 3.3 增强 role seed 测试，确认新增系统权限会被 seed 并进入超级管理员默认绑定。
- [x] 3.4 如更新 Swagger 注解，运行 Swagger 生成并检查生成文档 diff 是否仅包含预期接口说明变化。

## 4. Verification

- [x] 4.1 在 `user-service/` 运行 `go test ./internal/features/permission/application/rbacbaseline ./internal/features/permission/transport/http ./internal/features/permission/application/query ./internal/features/role/application/seed`。
- [x] 4.2 运行与 RBAC 相关的聚焦测试：`go test ./internal/features/role/... ./internal/features/permission/...`。
- [x] 4.3 在具备数据库和 Redis 的环境执行 `make seed-rbac`，再启动服务请求 `GET /api/v1/permissions/route-diff`，确认当前系统权限 URL 不再缺失。
- [x] 4.4 检查 `git diff`，确认没有修改 Ent 生成代码、migration、Casbin model 或无关 feature 行为。
