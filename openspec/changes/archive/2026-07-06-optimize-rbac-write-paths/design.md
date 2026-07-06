## Context

当前 RBAC 写路径存在两类性能浪费：权限和角色更新使用 `Update().Save(ctx)` 后再通过 `GetBy...` refetch 完整实体；角色权限和用户角色批量绑定使用循环单条 `Create().Save(ctx)`。这些行为集中在 `user-service/internal/features/permission` 和 `user-service/internal/features/role`，会影响在线 RBAC 管理接口、RBAC seed、系统绑定同步和超级管理员绑定相关路径。

现有 HTTP controller 和 command service 将写操作结果建模为完整实体响应，导致 store 即使已经完成写入也必须再查一次数据库。角色和权限的系统保护逻辑又会在部分命令执行前读取当前实体，使角色更新、角色启停和权限更新形成 `SELECT + UPDATE + SELECT` 的调用形态。绑定替换和系统绑定同步已经具备事务边界，但新增部分仍按记录数执行多次 INSERT。

本 change 不改变 RBAC 授权语义、系统 baseline、Casbin policy sync、用户角色缓存失效或数据库结构，只调整写侧返回契约和批量写入实现方式。

## Goals / Non-Goals

**Goals:**

- 消除权限和角色普通更新、启停 store 方法在写入成功后的实体 refetch。
- 将权限和角色更新、启用、停用 HTTP 成功响应调整为无实体响应，避免应用层为了响应体强制查询。
- 使用 Ent `CreateBulk` 优化角色权限和用户角色批量绑定写路径。
- 为 seed/ensure 这类幂等 bulk create 保留唯一冲突忽略语义，并保留事务回滚测试。
- 同步 OpenAPI 注解、生成物、应用层 mock 和相关单元测试。

**Non-Goals:**

- 不新增兼容旧响应体的分支、配置开关、版本化 endpoint 或临时 adapter。
- 不改变权限、角色、绑定、Casbin policy loader、Redis policy sync、用户角色缓存失效或超级管理员授权的业务语义。
- 不修改数据库 schema，不生成 Atlas migration。
- 不把 RBAC 业务逻辑迁移到 `common`、`internal/shared` 或 `internal/integration`。
- 不为测试新增生产专用 hook、fake 或冗余接口。

## Decisions

### 写命令返回无实体结果

权限和角色更新、启用、停用命令改为返回 `error` 或最小无实体结果，HTTP controller 成功后返回无响应体成功状态。这样 store 可以只依赖 `Update().Save(ctx)` 的 affected rows 判断 NotFound，成功时不再 refetch。

备选方案是继续返回实体并将 store 改为 `UpdateOneID`。该方案只在调用方已经拥有 Ent 内部 ID 时能减少查询；当前多数组件以外部 UUID 作为输入，若为 `UpdateOneID` 额外查询内部 ID，并不能减少往返。另一个备选方案是保留旧响应体并异步查询，但会增加复杂性且不符合本次不保留兼容方案的要求。

### 前置保护查询只保留业务必要性

系统角色保护和系统权限身份保护仍需要读取当前实体，因此 `UpdateRole`、`SetRoleActive` 和 `UpdatePermission` 的业务前置查询保留；移除的是 store 写入成功后的响应体 refetch。`SetPermissionActive` 当前没有系统保护前置查询，应维持单次 update 语义并通过 affected rows 映射 NotFound。

备选方案是让 store 内部自动读取当前实体并保护系统字段，但这会把 application/domain 规则下沉到 infrastructure，违反 feature 分层职责。

### 批量绑定使用 Ent CreateBulk

`RolePermissionStore.Replace`、`RolePermissionStore.EnsureSystemBindings`、`RolePermissionStore.SyncSystemBindings` 和 `UserRoleStore.Replace` 的新增绑定集合改为构造 builder slice 后调用 `CreateBulk(...).Save(ctx)`。空集合不调用 bulk create。事务性替换和同步继续使用现有事务边界，新增失败必须回滚删除或部分插入。

备选方案是保留循环但并发执行 INSERT。该方案会增加事务内并发复杂度，不能稳定改善数据库往返，也更难保证错误映射和回滚语义。

### Ent 生成启用 sql/upsert

`EnsureSystemBindings` 需要保留唯一冲突幂等处理；当前生成代码没有 `OnConflict/DoNothing` API，因此需要在 `user-service/ent/generate.go` 启用 `sql/upsert` 并执行 `make user-service-generate`。幂等 bulk create 使用唯一键冲突忽略，非唯一冲突错误仍返回并触发事务回滚或失败。

备选方案是在 bulk create 失败后回退到逐条插入来区分冲突。该方案保留了旧循环路径，无法落实本次最终不兼容方案。

### 变更归属保持在 feature 内

RBAC 写侧代码继续留在 permission 和 role feature：HTTP 返回契约在 transport/http，命令返回契约在 application/command，持久化优化在 infrastructure/postgres。Ent 生成约定属于 user-service module 的交付能力。`common`、`internal/shared`、`internal/integration`、deployments 和观测资产不承载本次业务代码。

## Risks / Trade-offs

- 旧客户端依赖 `200 + entity body` 会在更新、启用、停用接口上不兼容。缓解：明确作为 breaking change，同步 OpenAPI 生成物和测试，不提供旧响应体兼容层。
- `CreateBulk` 的错误粒度低于逐条插入。缓解：写入前继续完成角色、用户、权限存在性和权限 active 校验；幂等路径使用 `DoNothing` 仅忽略唯一冲突，其他错误仍失败。
- `CreateBulk` 在事务内任一非冲突错误必须回滚删除操作。缓解：为 Replace 和 Sync 增加失败回滚测试。
- Ent `sql/upsert` 生成物变更较大。缓解：执行 `make user-service-generate`，审查生成物 diff，并通过 `make verify` 暴露生成物 drift。
- Policy reload 或缓存失效遗漏会造成授权结果滞后。缓解：命令层保持写成功后的既有 notify 调用和相关测试 expectation。

## Migration Plan

1. 更新 OpenSpec delta、设计和任务后实施代码变更。
2. 调整 permission/role application port、command service、mock 生成物和测试。
3. 调整 permission/role HTTP controller 成功响应和 OpenAPI 注解。
4. 启用 Ent `sql/upsert` 并重新生成 Ent 代码。
5. 将角色权限和用户角色批量新增路径改为 `CreateBulk`，补充回滚和唯一冲突测试。
6. 执行 `make user-service-openapi-generate`、相关 Go 测试、`make user-service-architecture-lint`。
7. 暂存本次预期变更后执行 `make lint` 和 `make verify`。

回滚方式：在发布前回滚本 change 的代码、生成物、OpenAPI 和规格变更即可；没有数据库 migration 或部署资产需要回滚。发布后若旧客户端未适配，需要回滚服务版本和 OpenAPI 契约，不能通过配置恢复旧响应体。

## Open Questions

无。
