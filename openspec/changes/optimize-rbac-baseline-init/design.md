## Context

`user-service/internal/shared/rbacbaseline` 是 role、permission 共同消费的服务内系统 RBAC 基线。当前公开函数 `DefaultRoles()`、`DefaultPermissions()` 和 `DefaultRolePermissions()` 已经形成稳定调用契约，RBAC seed 依赖它们写入系统角色、权限投影和默认绑定。

现状中默认角色列表和默认角色权限绑定分别维护，`DefaultRolePermissions()` 直接把超级管理员展开为全部默认权限。当前行为正确，但未来新增默认系统角色时缺少一个集中维护位置，容易把绑定逻辑写成分支或按模块自动推导权限。

## Goals / Non-Goals

**Goals:**

- 保留 `rbacbaseline` 对外 API 和当前运行行为。
- 将默认角色元数据与该角色的默认权限来源集中到内部 catalog。
- 让超级管理员继续绑定全部默认权限。
- 为未来默认角色提供只通过显式 `PermissionID` 列表维护绑定的结构和注释示例。
- 调整测试，使测试语义覆盖已知角色、已知权限、重复绑定和超级管理员全量绑定。

**Non-Goals:**

- 不新增 `PermissionSet` 或权限集合别名。
- 不按 `Module`、model、read/write 或路由模式自动推导角色权限。
- 不新增默认角色 ID、常量或实际默认角色。
- 不修改 seed service、HTTP router、OpenAPI、Ent schema、Atlas migration 或部署资产。

## Decisions

1. 默认角色使用内部 `defaultRoleSpec` catalog 表达。

   `defaultRoleSpec` 内嵌 `RoleSpec`，并增加 `PermissionIDs func() []string`。`DefaultRoles()` 从 catalog 展开角色元数据，`DefaultRolePermissions()` 从同一个 catalog 展开默认绑定。这样新增默认角色时只需要新增一个 role block，角色元数据和绑定来源不会分散在两个函数中。

   备选方案是继续在 `DefaultRolePermissions()` 中为每个角色写分支。该方案改动更小，但每增加角色都会让绑定逻辑变长，且更容易夹带按模块推导的 helper。

2. 超级管理员保留 `allPermissionIDs()` 自动绑定。

   超级管理员是当前唯一默认角色，也是唯一需要拥有全部默认权限的系统角色。为它保留自动展开可以避免新增权限时漏绑超级管理员，同时不扩大到其他角色。

   备选方案是让超级管理员也显式列出全部权限 ID。该方案更一致，但每次新增权限都必须同步修改超级管理员绑定，反而增加漏绑风险。

3. 未来默认角色只允许显式权限 ID 列表。

   增加 `permissionIDs(ids ...string) func() []string`，用于未来角色复制示例时显式列出权限 ID。该 helper 只复制传入 slice，避免调用方共享底层数组；它不表达任何权限集合语义。

   备选方案是引入 `PermissionSet` 或 `modulePermissionIDs("user")`。这两类方案会把权限边界抽象得过粗，无法区分创建、修改、绑定、解绑、列表和详情等细粒度操作，存在误授权风险。

4. 测试验证基线关系而不是当前绑定数量。

   测试应验证所有 binding 引用已知角色和已知权限、绑定不重复、超级管理员拥有全部默认权限。这样当前只有超级管理员时行为仍被覆盖，未来新增默认角色时也不需要因为总绑定数变化而重写测试。

## Risks / Trade-offs

- 默认角色 catalog 使用函数返回权限 ID，未来开发者可能误用 `allPermissionIDs()` 给非超级管理员角色。Mitigation: 在 catalog 示例注释和 spec 中明确非超级管理员必须显式列出权限 ID，并用 code review 约束。
- `permissionIDs` helper 只是轻量复制函数，不验证权限 ID 是否存在。Mitigation: 单元测试统一校验所有默认绑定引用 `DefaultPermissions()` 中的已知权限。
- 本 change 不修改 seed 或数据库数据，无法防止数据库中历史手工绑定问题。Mitigation: 范围限定为代码基线维护结构，seed 继续按当前基线幂等写入。

## Migration Plan

无需数据库 migration、OpenAPI 生成或部署编排变更。代码发布后，`rbac seed` 的默认角色、权限和绑定输出保持一致；如需回滚，可回退本 change 的 `rbacbaseline` 代码和测试，不涉及数据回滚。

## Open Questions

无。
