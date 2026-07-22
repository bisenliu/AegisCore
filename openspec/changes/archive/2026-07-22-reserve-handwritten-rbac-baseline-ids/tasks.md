## 1. 系统 ID 常量

- [x] 1.1 检查 `user-service/internal/shared/rbacbaseline/ids.go` 当前 UUIDv5 namespace、semantic name 和常量引用，确认需要删除或替换的符号。
- [x] 1.2 将 `BootstrapSuperAdminUserID`、`SuperAdminRoleID` 和全部 baseline permission ID 改为手写保留 UUID 常量，格式为 `00000000-0000-0000-0000-TTMMSSSSSSSS`。
- [x] 1.3 为 `ids.go` 补充中文注释，集中记录 type、module、sequence 编码规则，并在每个系统 ID 常量注释中保留 semantic，且不保留 UUIDv5 生成说明。
- [x] 1.4 删除不再需要的 UUIDv5 namespace、semantic name 生成 helper 或生成算法相关常量，确认生产代码不依赖动态系统 ID 生成逻辑。

## 2. RBAC 基线引用

- [x] 2.1 更新 `DefaultPermissions()`，确保所有 `PermissionSpec.PermissionID` 引用 `rbacbaseline` permission ID 常量，且不内联 UUID 字符串。
- [x] 2.2 检查 `DefaultRoles()` 和 `DefaultRolePermissions()`，确保系统角色、bootstrap 用户相关流程和默认绑定只引用 `rbacbaseline` 固化常量。
- [x] 2.3 搜索 seed、bootstrap、HTTP runtime 和 policy loader 相关代码，确认系统基线路径不调用 `id.NewUUID()`、UUIDv5 或其他动态生成逻辑生成系统 ID。
- [x] 2.4 确认普通用户、普通角色和运行时业务数据创建路径仍使用当前运行时 ID 生成策略，未改用系统保留 ID 格式。

## 3. 测试更新

- [x] 3.1 更新 `user-service/internal/shared/rbacbaseline/ids_test.go`，移除 UUIDv5 namespace、UUID version 5 和 `uuid.NewSHA1` 复算校验。
- [x] 3.2 手写维护 `systemIDCases()`，登记所有系统用户、系统角色和系统权限 ID 的 name、id、typeCode 和 module。
- [x] 3.3 实现或更新 `TestSystemIDsUseReservedFormat`，校验 UUID 可解析且匹配 `^00000000-0000-0000-0000-[0-9]{12}$`。
- [x] 3.4 实现或更新 `TestSystemIDsMatchTypeModule`，校验最后 12 位中的 type/module 编码和 sequence 非 `00000000`。
- [x] 3.5 实现或更新 `TestSystemIDsGloballyUnique`、`TestDefaultPermissionsUseRegisteredSystemIDs` 和 `TestDefaultRolePermissionsUseRegisteredSystemIDs`，校验全局唯一、默认权限登记和默认绑定引用登记。
- [x] 3.6 如现有测试名称或 helper 与新规则冲突，按新规则调整命名并保持测试只校验契约，不重新引入生成算法。

## 4. 验证与交付

- [x] 4.1 运行 `go test ./user-service/internal/shared/rbacbaseline/...`，确认 RBAC baseline 单元测试通过。
- [x] 4.2 运行受影响 role/permission/bootstrap 相关包测试，确认 seed、默认绑定和 bootstrap 引用未回归。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认 shared 边界和架构规则未被破坏。
- [x] 4.4 检查本次变更未修改 OpenAPI、数据库 schema、migration、部署或观测生成物；如出现非预期 diff，定位并修正。
- [x] 4.5 将本次预期代码和 OpenSpec artifact 变更加到暂存区，再运行 `make lint`。
- [x] 4.6 在暂存本次预期变更后运行 `make verify`，确认最终验证通过且没有未预期 drift。
