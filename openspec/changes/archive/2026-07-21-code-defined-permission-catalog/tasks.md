## 1. 引用盘点与权限基线收敛

- [x] 1.1 使用 `rg` 盘点 permission `Active`/`IsSystem`、`active`/`is_system`、两个索引、6 个待删除路由、permission command、route diff、store port、mock 和观测引用，记录 role 模块仍消费的查询端口及 6 条权限的稳定 UUID
- [x] 1.2 从 `rbacbaseline.DefaultPermissions()` 删除 6 个废弃权限定义，保持其余权限稳定 ID 不变，并更新基线唯一性、字段校验和默认角色绑定测试
- [x] 1.3 将 `SeedPermissionInput` 收敛为 `PermissionID`、`Name`、`Description`、`Module`、`HTTPMethod`、`PathTemplate`，把 `UpsertSystemPermission` 及实现/测试重命名为 `UpsertPermission`

## 2. Permission 模型与查询能力清理

- [x] 2.1 从 permission domain entity、input、list filter、DTO、response、mapper、validator 和相关测试/mock 中删除 `Active`、`IsSystem`、`active`、`is_system` 与 `system`，同时确认 `Role.Active` 和 `Role.IsSystem` 仍完整保留
- [x] 2.2 删除 `CreatePermission`、`UpdatePermission`、`EnablePermission`、`DisablePermission` command 能力以及仅由其使用的输入、错误、validator 调用、notifier 依赖和 mock，保留列表、有效权限、seed 与 role lookup 所需最小 port
- [x] 2.3 删除 PostgreSQL permission store 的 Create、Update、SetActive 和 active/is_system predicate，调整列表、有效权限、seed upsert 与 lookup 查询及其单元/集成测试

## 3. Role 绑定与 Casbin policy 调整

- [x] 3.1 将角色权限绑定校验改为权限存在即可绑定，把 `getLockedActivePermission...` 等 helper 和 port 重命名为普通 permission lookup，并覆盖普通角色绑定任意基线权限、不存在权限回滚和完整替换事务测试
- [x] 3.2 从用户有效权限聚合删除权限 active 过滤，保持启用角色过滤、结果去重和无权限状态字段，并更新 permission/role 查询测试
- [x] 3.3 从 Casbin policy loader 删除 `permission.ActiveEQ(true)`，保留 `role.ActiveEQ(true)`、role_permissions/permissions policy 构造、超级管理员 wildcard 和 fail-closed 语义，并更新 loader/authorization/policy sync 测试

## 4. HTTP API 与生产装配收缩

- [x] 4.1 将 permission `RegisterRoutes` 收敛为 `GET ""` 和 `GET "/users/:user_id/effective"`，删除创建、详情、更新、启停和 route diff controller/request/response/OpenAPI 注解
- [x] 4.2 更新 permission controller、mapper 和 router 注册测试，断言只有两个权限接口存在、6 个废弃接口均未注册且保留接口仍经过认证与 RBAC middleware
- [x] 4.3 删除只服务于公开 route diff 的 application query、`RouteCatalogScanner` 生产实现/接口、metrics 和测试，并清理 permission Fx providers、route registrar 与父级 router 的悬空依赖
- [x] 4.4 使用 `rg` 检查 route diff metrics 是否被 Prometheus/Grafana 资产引用；若存在则做最小同步删除并运行 `make compose-dashboard-check`

## 5. 路由与代码基线 CI 门禁

- [x] 5.1 基于生产 route registrars 构建真实 Gin route graph，提取 `/api/v1` 下需要授权的 HTTP method + route template，并明确排除认证公开接口、会话控制接口和 `OPTIONS`
- [x] 5.2 实现 route graph 与 `rbacbaseline.DefaultPermissions()` 的双向一致性测试，覆盖 missing、stale、排除路由及稳定 permission ID 对应 method/path 的场景，确保不引入运行时自动修复或 application 对 Gin 的依赖
- [x] 5.3 运行 router、rbacbaseline、permission、role 和 Casbin 相关 Go 包测试并修复回归

## 6. Ent schema、Atlas migration 与 E2E 数据

- [x] 6.1 从 Permission Ent schema 删除 `active`、`is_system` 及 `permission_active_permission_id`、`permission_is_system_permission_id` 索引，运行 `make user-service-generate` 并审查 Ent 生成代码仅包含预期变化
- [x] 6.2 生成并审查 Atlas migration：按已核对的 6 个稳定 UUID 先删除 `role_permissions`、再删除 `permissions`，随后删除两个索引和两列，保证目标数据不存在时安全且不依赖运行时 migration
- [x] 6.3 更新 E2E 初始化 SQL、permission/role fixtures 和数据库测试以匹配新 schema、剩余基线权限和绑定关系
- [x] 6.4 运行 `make user-service-migrate-validate`，验证 migration 顺序、外键清理和 schema 一致性

## 7. OpenAPI、规格与项目文档

- [x] 7.1 运行 `make user-service-openapi-generate`，确认 OpenAPI 不再包含 6 个废弃 endpoint 或 Permission 的 active/system 字段，且两个保留接口契约正确
- [x] 7.2 更新 `docs/PRODUCT.md` 与 `docs/ARCHITECTURE.md`，说明权限代码权威、数据库只读投影、受控删除、seed 后 reload/重启顺序和 CI route graph 门禁
- [x] 7.3 将本 change 的 `rbac-access-control` delta 同步到长期主规格 `openspec/specs/rbac-access-control/spec.md`，确保权限只读语义与 Role.Active/IsSystem 保留要求一致
- [x] 7.4 运行 `make user-service-architecture-lint`，并用 `rg` 全仓确认 Permission 全链路不再出现 Active/IsSystem、权限 active/system 字段、废弃接口和无用 route diff 生产装配

## 8. 最终验证与交付

- [x] 8.1 运行完整相关测试与 E2E，验证权限列表、用户有效权限、普通角色权限绑定、启用角色 Casbin policy、policy reload/readiness 和路由基线不一致失败场景
- [x] 8.2 重新运行 `make user-service-generate`、`make user-service-openapi-generate` 和相关生成检查，使用 `git diff` 审查生成物并确认二次生成没有额外 drift
- [x] 8.3 审查工作树并只暂存本 change 的预期代码、migration、生成物、测试、文档和 OpenSpec 文件
- [x] 8.4 在预期变更已暂存后运行 `make lint`，通过后再运行 `make verify`；任一命令未运行或失败时保持任务未完成
