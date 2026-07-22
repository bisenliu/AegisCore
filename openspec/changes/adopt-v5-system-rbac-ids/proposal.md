## Why

AegisCore 作为基础框架被重命名或复用为新项目时，系统内置 RBAC 角色、权限和 bootstrap 用户 ID 必须保持稳定、可审计且不随项目展示名漂移。当前系统内置 ID 的来源、固化规则和重命名边界缺少统一规格，容易破坏 RBAC seed 幂等性、bootstrap 一次性语义和历史审计链路。

## What Changes

- 将系统内置 RBAC/bootstrap ID 明确为 UUID v5 生成后固化的常量，包括 `SuperAdminRoleID`、`BootstrapSuperAdminUserID` 和全部 baseline permission ID。
- 明确普通运行时业务实体继续使用 `common/runtime/id.NewUUID()` 生成 UUID v7，系统基线 ID 禁止使用运行时随机 ID。
- 在 `user-service/internal/shared/rbacbaseline/` 统一管理系统 ID 常量、UUID v5 namespace 和 semantic name 约定。
- 要求 `DefaultRoles()`、`DefaultPermissions()` 和 `DefaultRolePermissions()` 引用 ID 常量，不得内联系统 UUID 字符串。
- 要求 bootstrap super admin 逻辑消费 `rbacbaseline.BootstrapSuperAdminUserID`，不再在 bootstrap 包内私有定义系统用户 ID。
- 增加 ID 一致性测试，校验固化常量可解析、版本为 v5、与 namespace + semantic name 的 UUID v5 结果一致且无重复。
- 明确新项目初始化可以生成新的 `SystemIDNamespace` 和系统 ID 常量；已有项目重命名不得默认重算系统 ID 或修改既有数据库 RBAC 数据。
- 不新增数据库表，不新增 Atlas migration；本 change 不提供旧数据库中历史系统 ID 的原地迁移。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 修改系统内置角色、权限和 bootstrap 用户 ID 的稳定性、生成来源、固化位置和 seed 引用要求。
- `delivery-operations`: 补充基础框架初始化新项目与已有项目重命名时对系统内置 ID 的操作边界。

## Impact

- 代码影响：`user-service/internal/shared/rbacbaseline/`、RBAC seed、permission baseline、role baseline、bootstrap super admin application、相关单元测试。
- 数据影响：不新增 schema，不新增 migration；全新数据库 seed 后会使用新的 UUID v5 固化系统 ID。已有数据库原地升级不在本 change 范围内。
- 安全影响：提升系统内置 RBAC 数据的幂等性、可审计性和重命名稳定性，避免因随机 ID 或项目名漂移导致授权基线不一致。
- 运维影响：项目重命名脚本不得默认重算系统 ID；新项目初始化脚本可以一次性生成 namespace 和固化 ID，但不得连接或修改数据库。
- API/OpenAPI 影响：HTTP API 路径、请求和响应契约不变。
