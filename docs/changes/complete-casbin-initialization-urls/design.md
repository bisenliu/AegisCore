## Context

用户服务的 HTTP route graph 在 `/api/v1` 下分成三类：公有 auth 路由、仅需要 JWT 认证的 auth session 路由，以及 JWT 后继续进入 Casbin RBAC 授权的业务路由。当前 `RouteCatalogScanner` 将 `/api/v1/*` 中除 auth、`OPTIONS`、健康检查和 Swagger 之外的业务路由视为可授权路由；Casbin middleware 使用 Gin `c.FullPath()` 作为 object，使用 HTTP method 作为 action。

系统权限初始化由 `rbac seed` 调用 role seed application service，读取 `permission/application/rbacbaseline.DefaultPermissions()` 和 `DefaultRolePermissions()`，再通过 permission PostgreSQL adapter 幂等 upsert 到权限目录。Casbin policy loader 之后从 `roles`、`permissions`、`user_roles`、`role_permissions` 加载启用记录，并额外给内置超级管理员角色追加 wildcard policy。

本变更只补齐系统初始化 URL catalog 与测试/文档，不修改授权模型、seed 执行方式或运行时 policy reload 机制。

## Goals / Non-Goals

**Goals:**

- 让 `DefaultPermissions()` 覆盖当前所有已注册且可授权的业务路由。
- 使用与 Gin 注册完全一致的 path template，例如 `:user_id`、`:role_id`、`:permission_id`。
- 使用稳定 UUID 字符串作为 `permission_id`，避免后续 seed 更新产生新权限身份。
- 为每条系统权限设置清晰名称、模块和说明，便于权限目录列表展示和运营识别。
- 让 `DefaultRolePermissions()` 默认把所有系统权限绑定给 `SuperAdminRoleID`。
- 通过测试防止新增受保护路由后忘记更新初始化 catalog。
- 保持与现有 permission command policy change notifier、role binding policy refresh 和 Casbin loader 行为兼容。

**Non-Goals:**

- 不新增 HTTP 接口。
- 不自动给普通角色绑定新权限。
- 不把 auth 登录、refresh、改密、登出、健康检查或 Swagger 纳入 RBAC catalog。
- 不新增 `casbin_rules` 表或改变 Casbin model。
- 不在 HTTP server 启动时自动执行 seed。
- 不通过 route diff 自动写入权限目录。

## Interface Catalog

以下接口是当前 route graph 中应进入 Casbin 初始化 catalog 的 URL。所有接口均使用统一 response envelope：成功响应为 `success=true`、`code`、`message`、`data`，失败响应为 `success=false`、`code`、`message`，并由现有 `common/http/response` 输出。

| 模块 | 方法 | 路径模板 | 用途 | 入参 | data 返回结构 |
|---|---|---|---|---|---|
| user | GET | `/api/v1/users` | 分页查询用户资料 | query: `cursor`, `page_size`, `nickname`, `username`, `status` | `UserListResponseDoc`，包含 `items` 和 `pagination` |
| user | POST | `/api/v1/users` | 创建用户资料和初始凭据 | JSON: `nickname`, `username`, `password`, `status` | `UserResponseDoc` |
| user | GET | `/api/v1/users/:user_id` | 查询用户资料详情 | path: `user_id` | `UserResponseDoc` |
| permission | GET | `/api/v1/permissions` | 分页查询权限目录 | query: `cursor`, `page_size`, `module`, `http_method`, `active`, `system` | `PermissionListResponseDoc` |
| permission | POST | `/api/v1/permissions` | 创建正式权限目录记录 | JSON: `name`, `description`, `module`, `http_method`, `path_template`, `active`, `system` | `PermissionResponse` |
| permission | GET | `/api/v1/permissions/route-diff` | 只读查询已注册路由与权限目录差异 | 无 | `RouteDiffResponse` |
| permission | GET | `/api/v1/permissions/users/:user_id/effective` | 查询用户当前有效权限集合 | path: `user_id` | `[]PermissionResponse` |
| permission | GET | `/api/v1/permissions/:permission_id` | 查询权限详情 | path: `permission_id` | `PermissionResponse` |
| permission | PUT | `/api/v1/permissions/:permission_id` | 更新权限目录记录 | path: `permission_id`; JSON: `name`, `description`, `module`, `http_method`, `path_template`, `active` | `PermissionResponse` |
| permission | POST | `/api/v1/permissions/:permission_id/enable` | 启用权限目录记录 | path: `permission_id` | `PermissionResponse` |
| permission | POST | `/api/v1/permissions/:permission_id/disable` | 停用权限目录记录 | path: `permission_id` | `PermissionResponse` |
| role | GET | `/api/v1/roles` | 分页查询角色 | query: `cursor`, `page_size`, `active`, `system` | `RoleListResponseDoc` |
| role | POST | `/api/v1/roles` | 创建角色 | JSON: `name`, `description`, `active`, `system` | `RoleResponse` |
| role | GET | `/api/v1/roles/:role_id` | 查询角色详情 | path: `role_id` | `RoleResponse` |
| role | PATCH | `/api/v1/roles/:role_id` | 更新角色元数据 | path: `role_id`; JSON: `name`, `description`, `active` | `RoleResponse` |
| role | PATCH | `/api/v1/roles/:role_id/status` | 启用或停用角色 | path: `role_id`; JSON: `active` | `RoleResponse` |
| role | GET | `/api/v1/users/:user_id/roles` | 查询用户绑定角色 | path: `user_id` | `[]RoleResponse` |
| role | PUT | `/api/v1/users/:user_id/roles` | 全量替换用户角色集合 | path: `user_id`; JSON: `role_ids` | `[]RoleResponse` |
| role | POST | `/api/v1/users/:user_id/roles` | 增量绑定用户角色 | path: `user_id`; JSON: `role_id` | `[]RoleResponse` |
| role | DELETE | `/api/v1/users/:user_id/roles/:role_id` | 解绑用户角色 | path: `user_id`, `role_id` | `[]RoleResponse` |
| role | GET | `/api/v1/roles/:role_id/permissions` | 查询角色权限 | path: `role_id` | `[]rolehttp.PermissionResponse` |
| role | PUT | `/api/v1/roles/:role_id/permissions` | 全量替换角色权限集合 | path: `role_id`; JSON: `permission_ids` | `[]rolehttp.PermissionResponse` |
| role | POST | `/api/v1/roles/:role_id/permissions` | 增量绑定角色权限 | path: `role_id`; JSON: `permission_id` | `[]rolehttp.PermissionResponse` |
| role | DELETE | `/api/v1/roles/:role_id/permissions/:permission_id` | 解绑角色权限 | path: `role_id`, `permission_id` | `[]rolehttp.PermissionResponse` |

Excluded interfaces:

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/change-password`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/logout-all`
- `/healthz`
- Swagger routes and redirects
- `OPTIONS`

## Decisions

### Decision: Catalog 直接枚举当前可授权路由

`DefaultPermissions()` 使用显式常量枚举每条系统权限，而不是运行时扫描 Gin route 后自动生成。

Rationale: 初始化权限需要稳定 `permission_id`、名称、模块和说明；运行时扫描只能发现 method/path，无法安全生成稳定业务身份和运营展示字段。

### Decision: Path template 以 Gin route template 为唯一匹配格式

系统权限中的 `PathTemplate` 使用 `/api/v1/users/:user_id` 等 Gin 模板，不使用 Swagger `{user_id}` 或真实 URL `/api/v1/users/<uuid>`。

Rationale: RBAC middleware 调用 `c.FullPath()`，Casbin policy loader 从权限目录读取 `path_template`，两者必须字符串一致才能命中。

### Decision: 超级管理员绑定覆盖全部系统权限

`DefaultRolePermissions()` 为 `SuperAdminRoleID` 绑定 `DefaultPermissions()` 中所有权限，同时 Casbin loader 继续追加 wildcard policy。

Rationale: 显式绑定让权限目录、seed 结果和 route diff 对超级管理员默认授权可观察；wildcard policy 继续作为内置兜底，保证超级管理员不因 catalog 漏项阻断访问。

### Decision: Catalog 测试对齐 route scanner

新增测试应构造当前 route graph 或维护同一份期望 route set，断言 `DefaultPermissions()` 的 method/path 与可授权路由完全一致。

Rationale: 现有 route diff 是运行时诊断；单元测试能在新增路由但忘记更新初始化 catalog 时更早失败。

## Implementation Notes

- 可以按模块保留当前 `00000000-0000-0000-0000-00000001xxxx` 用户权限 ID 区间，并为 permission、role、user-role、role-permission 分配新的稳定区间，例如 `00000002xxxx`、`00000003xxxx`、`00000004xxxx`、`00000005xxxx`。
- `PermissionSpec.Module` 建议使用 `user`、`permission`、`role`，用户角色绑定可归入 `role` 模块，名称中体现“用户角色”。
- 变更 `DefaultPermissions()` 后同步更新 `DefaultRolePermissions()`，避免 seed catalog 校验失败。
- 保持 permission/role/user HTTP DTO 不变；本变更不需要新增 request/response 类型。
- 如果发现 Swagger `@Router` 与真实 Gin path template 不一致，优先只更新注解和生成文档，不改变真实路由。

## Risks / Trade-offs

- [Risk] 新增 catalog 权限后 seed 默认只补齐，不会删除旧权限，可能保留历史 stale 权限。Mitigation: 发布后运行 route diff；需要精确同步系统角色绑定时显式使用 `--sync-system-bindings`。
- [Risk] 权限 ID 人工分配出错导致重复。Mitigation: 保留并增强 `catalog_test.go` 的 duplicate ID 和 duplicate route 校验。
- [Risk] route scanner 和 catalog 过滤边界不一致。Mitigation: 为 `RouteCatalogScanner` 的 auth 排除和业务路由发现增加测试。
- [Risk] 修改系统权限 active 状态影响授权。Mitigation: `rbac seed` 默认不重新启用已停用系统权限，只有 `--reactivate-system` 明确恢复。

## Verification

- `go test ./internal/features/permission/application/rbacbaseline`
- `go test ./internal/features/permission/transport/http ./internal/features/permission/application/query`
- `go test ./internal/features/role/application/seed`
- 在可连接 PostgreSQL/Redis 的环境执行：
  - `make seed-rbac`
  - 启动服务后请求 `GET /api/v1/permissions/route-diff`，确认当前受保护路由不再出现在 `missing_in_permissions`。
