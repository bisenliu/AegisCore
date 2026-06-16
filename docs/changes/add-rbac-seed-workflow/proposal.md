## Why

RBAC 默认角色、权限和系统角色授权需要一个显式、幂等、可重复执行的维护流程，以便在迁移 schema 后、HTTP 服务启动前稳定初始化或更新系统访问控制数据。当前如果缺少统一 seed 入口，新增受保护路由、系统角色或首个超级管理员绑定容易依赖手工 SQL 或启动时隐式副作用，增加部署不可重复和权限漂移风险。

## What Changes

- 新增显式 RBAC seed 能力，用于写入和更新默认系统角色、默认系统权限、系统角色权限绑定。
- 新增 `aegiscore-user-services rbac seed` CLI 命令，并提供 `make seed-rbac` 包装入口。
- 新增代码 catalog 管理默认角色、默认权限和默认系统角色权限绑定。
- seed 按 `role_id` 幂等 upsert 系统角色，按 `permission_id` 或 `http_method + path_template` 幂等 upsert 系统权限，并补齐系统角色权限绑定。
- seed 默认不在 HTTP server 启动时执行，不删除 catalog 中移除的数据，不自动给普通自定义角色授权，不覆盖 `active=false`。
- seed 提供 `--reactivate-system` 选项用于显式恢复系统角色或系统权限启用状态。
- seed 提供 `--sync-system-bindings` 选项用于按 catalog 精确同步系统角色权限绑定。
- 新增 `aegiscore-user-services rbac assign-super-admin --user-id <uuid>` CLI 命令，用于显式把首个超级管理员角色绑定给指定用户。
- 明确部署顺序为 migrate schema -> seed RBAC data -> start HTTP server。

## Capabilities

### New Capabilities

- `rbac-seed-workflow`: 定义 RBAC 系统角色、系统权限、系统角色权限绑定和超级管理员用户绑定的显式 seed 与更新流程。

### Modified Capabilities

无。

## Impact

- 影响 `user-service/cmd/` 的 Cobra CLI 命令结构，新增 RBAC seed 与超级管理员绑定入口。
- 影响 `user-service/internal/features/role/` 和 `user-service/internal/features/permission/`，新增 application catalog 与 seed 所需 application/infrastructure 能力。
- 影响 `Makefile`，新增 `seed-rbac` 包装命令。
- 影响部署和运维流程，要求在启动 HTTP server 前显式执行 RBAC seed，并在新增受保护路由后通过 catalog + seed + route-diff 完成权限落库和校验。
- 不引入新的外部依赖，不新增 `casbin_rules` 表，不实现审计日志、菜单权限、多租户或角色继承。
