## Context

角色权限绑定 seed 入口位于 role feature。`SeedRolePermissionStore` 由 `RolePermissionStore` 实现，`EnsureSystemBindings` 和 `SyncSystemBindings` 都会先调用 `permissionsByExternalIDs`，再基于权限表内部 ID 判断和写入 `role_permissions` 绑定。

当前 `permissionsByExternalIDs` 在循环中调用 `getPermissionByExternalID`，每个外部 `permission_id` 都会触发一次 Ent 查询。这个行为确实存在于 seed 路径，但 HTTP `ReplaceRolePermissions` 传入 `RolePermissionStore.Replace` 前已经在 application 层解析为 `PermissionReference`，不经过该 helper。

## Goals / Non-Goals

**Goals:**

- 将 seed 权限解析压缩为单次批量查询。
- 保留现有去重、缺失权限错误和绑定写入语义。
- 保持 role infrastructure 边界，不引入 permission infrastructure 反向依赖。
- 为批量解析增加回归测试，防止重新退化为 N 次查询或吞掉缺失权限。

**Non-Goals:**

- 不调整 HTTP replace 的逐个 `PermissionLookup.GetActiveByPermissionID` 校验路径。
- 不把批量权限查询端口提升到 application 层。
- 不改变 seed catalog、RBAC policy reload、多实例同步或 Casbin loader 行为。
- 不修改事务边界；当前权限解析仍在 `SyncSystemBindings` 显式事务开始前完成。

## Design

### Batch Permission Resolution

`permissionsByExternalIDs` 继续负责把外部 UUID 列表解析为 `*ent.Permission` 列表。实现时先按输入顺序去重，空输入直接返回空切片，然后使用 Ent 生成的 `entpermission.PermissionIDIn(uniqueIDs...)` 一次查询权限表。

批量查询返回后，以 `permission.PermissionID` 构建 map 并校验每个去重后的输入 ID 都存在。这样不仅能校验返回数量，也能明确识别缺失的外部 ID，避免只用长度比较时被异常重复返回或数据异常掩盖。

### Result Ordering

返回值应按去重后的输入顺序组装，而不是依赖数据库返回顺序。当前调用方只依赖集合内容，不依赖排序；保持输入顺序可以让行为更稳定，也便于测试。

### Error Semantics

缺失权限时继续返回 `roledomain.ErrRolePermissionNotFound`，必要时用 `fmt.Errorf("%w: permission_id %s", roledomain.ErrRolePermissionNotFound, missingID.String())` 增加诊断上下文。其他数据库错误继续包装为查询失败错误。

### Transaction Boundary

`EnsureSystemBindings` 无显式事务；`SyncSystemBindings` 当前在权限解析和现有绑定读取之后才开启事务。本变更不移动事务边界，只减少事务前的权限解析数据库往返次数。

## Validation

- 新增或更新 PostgreSQL adapter 测试，覆盖多个权限 ID 时能完成 seed 绑定。
- 覆盖重复 `permission_id` 输入只产生一次解析结果和一次绑定判断。
- 覆盖缺失 `permission_id` 返回角色权限 not found 语义。
- 运行 `go test ./...` 于 `user-service/`。
- 如测试环境支持真实数据库，补充运行 RBAC seed 相关集成测试。
