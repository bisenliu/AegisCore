## Context

RBAC baseline 当前由 `user-service/internal/shared/rbacbaseline` 维护系统角色、系统权限、默认绑定和 bootstrap 用户 ID。现有规格要求系统 ID 由 UUIDv5 semantic name 生成后固化，但该模型仍需要维护 namespace、semantic name 与 UUIDv5 计算关系，并鼓励测试复算算法结果。新的约束要求完全手写保留 UUID：不使用 UUIDv5，不使用 `go:generate`，不提前给尚未进入 baseline 的权限模块分配编号。

该变更只影响 `user-service` 内的 RBAC baseline 常量和相关测试，不改变 `common` runtime ID primitive，不改变 HTTP API、OpenAPI、数据库 schema、migration、部署清单或观测资产。系统内置 ID 属于安全与授权数据契约，必须在 `internal/shared/rbacbaseline` 边界内集中维护，seed、bootstrap、policy loader 和 HTTP runtime 只能引用这些常量。

## Goals / Non-Goals

**Goals:**

- 将所有 RBAC 系统保留 ID 手写固化到 `user-service/internal/shared/rbacbaseline/ids.go`。
- 使用 `00000000-0000-0000-0000-TTMMSSSSSSSS` 作为唯一保留格式，并明确类型、模块和序号规则。
- 只为当前真实存在于 `DefaultPermissions()` 的权限模块分配连续 `MM`，后续模块按首次进入 baseline 的顺序追加并固化。
- 让 `DefaultPermissions()`、`DefaultRoles()`、`DefaultRolePermissions()` 和 bootstrap/seed 代码全部引用 `rbacbaseline` 常量，不内联 UUID，也不动态生成系统 ID。
- 更新测试，验证格式、类型模块编码、sequence 非零、全局唯一、默认权限和默认绑定引用登记。

**Non-Goals:**

- 不引入 UUIDv5、UUID namespace、semantic name 复算、`go:generate` 或自动分配工具。
- 不预分配 `auth`、`audit`、`tenant` 等未来模块编号。
- 不改变普通业务用户、普通角色或运行时创建数据的 UUID v7 生成策略。
- 不改变数据库结构、seed 幂等语义、bootstrap 事务语义、RBAC HTTP API 或 Casbin policy 同步机制。

## Decisions

1. 系统 ID 常量唯一来源为 `rbacbaseline/ids.go`。

   选择：所有系统内置用户、系统角色和系统权限 ID 只在 `user-service/internal/shared/rbacbaseline/ids.go` 中手写声明，其他代码通过常量引用。

   理由：`internal/shared/rbacbaseline` 已是 role/permission 共同消费的稳定服务内业务内核，集中维护可避免 seed、bootstrap、policy 或测试各自复制 ID。

   备选方案：在 seed、bootstrap 或各 feature 内局部定义 ID。放弃原因是会造成系统数据契约分散，增加重复和漂移风险。

2. 使用手写保留 UUID 格式而不是 UUIDv5。

   选择：常量值使用 `00000000-0000-0000-0000-TTMMSSSSSSSS`，其中 `TT` 为实体类型，`MM` 为模块编号，`SSSSSSSS` 为同一 `TT+MM` 下递增序号。

   理由：该格式仍可通过 UUID parser 校验，同时人眼可直接识别实体类型、模块和顺序，不依赖 namespace、hash 算法或生成工具。

   备选方案：保留 UUIDv5 并强化注释或测试。放弃原因是实现者仍需理解并维护算法关系，且与“不使用 UUIDv5”的最终方案冲突。

3. 权限模块编号只按真实进入 baseline 的顺序固化。

   选择：当前权限模块分配为 `01=user`、`02=permission`、`03=role`、`04=user-role`、`05=role-permission`；后续新增模块按首次加入 `DefaultPermissions()` 的顺序使用下一个可用 `MM`。

   理由：避免为未来可能不存在或顺序不确定的模块提前占号，保持编号表只表达已经发布的 baseline 契约。

   备选方案：提前预留常见模块如 `auth`、`audit`、`tenant`。放弃原因是会制造未实现能力的隐式契约，并可能在真实需求出现时造成编号语义不一致。

4. 测试登记所有系统 ID，而不是复算生成算法。

   选择：`ids_test.go` 通过手写 `systemIDCases()` 登记系统 ID、类型编号和模块编号，并校验 UUID 解析、保留格式、type/module、sequence 非零、全局唯一和默认 baseline 引用关系。

   理由：系统 ID 是手写契约，测试应防止格式和引用漂移，而不是重新引入生成算法。

   备选方案：测试中动态从常量名或权限模块推导编号。放弃原因是会隐藏新增权限时必须显式登记和审查的维护动作。

## Risks / Trade-offs

- [Risk] 手写 ID 可能因人工输入错误导致重复或 type/module 编码错误。→ Mitigation：增加全局唯一、格式、type/module 和 sequence 非零测试，并要求所有权限 ID 登记在 `registeredPermissionIDs()`。
- [Risk] 删除权限后误复用旧 ID。→ Mitigation：规格明确已发布或已删除 ID 不得复用；实现时在 `ids.go` 注释和测试登记中保留审查入口。
- [Risk] 新增权限模块时开发者可能提前选择固定未来编号。→ Mitigation：规格和测试要求只为进入 `DefaultPermissions()` 的模块分配编号，并按首次进入顺序追加。
- [Risk] 普通业务数据误用保留格式。→ Mitigation：保留运行时 ID 与系统 ID 分离要求，普通创建路径继续使用 `common/runtime/id.NewUUID()`，不得复用系统常量或保留格式。

## Migration Plan

1. 更新 `rbacbaseline/ids.go` 中已有系统用户、角色和权限常量为手写保留 UUID。
2. 更新 `DefaultPermissions()`、`DefaultRoles()`、`DefaultRolePermissions()`、seed 和 bootstrap 引用，确保不内联 UUID、不动态生成系统 ID。
3. 更新 `ids_test.go`，移除 UUIDv5 namespace 和复算校验，改为保留格式与登记引用校验。
4. 运行相关包测试和架构 lint：`go test ./user-service/internal/shared/rbacbaseline/...`、`make user-service-architecture-lint`。

回滚方式：如果实现前发现编号表需要调整，直接修改 change artifacts；如果实现后尚未发布，可在同一变更中调整常量和测试。发布后系统 ID 不得修改或复用，回滚只能通过恢复旧代码并配合数据层人工审查既有系统记录。

## Open Questions

无。
