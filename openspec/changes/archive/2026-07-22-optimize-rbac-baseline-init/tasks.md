## 1. RBAC baseline catalog 重构

- [x] 1.1 在 `user-service/internal/shared/rbacbaseline/catalog.go` 中新增内部默认角色 catalog，让 `DefaultRoles()` 和 `DefaultRolePermissions()` 从同一 catalog 展开。
- [x] 1.2 增加 `permissionIDs` 和 `allPermissionIDs` helper，保留超级管理员全量权限绑定，并为未来默认角色提供显式权限 ID 注释示例。

## 2. 测试调整

- [x] 2.1 更新 `user-service/internal/shared/rbacbaseline/catalog_test.go`，校验默认绑定引用已知角色、已知权限且不重复。
- [x] 2.2 保留超级管理员绑定全部默认权限的测试语义，并避免依赖当前总 binding 数量。

## 3. 验证与收尾

- [x] 3.1 运行 `go test ./internal/shared/rbacbaseline` 验证目标包。
- [x] 3.2 运行 `make user-service-architecture-lint` 验证共享边界和 OpenSpec 相关结构规则。
- [x] 3.3 暂存本次预期代码和 OpenSpec change 产物，排除 runtime 文件和既有无关变更。
- [x] 3.4 运行 `make lint` 和 `make verify`，如因既有无关工作区变更导致最终 diff 检查失败，记录排除说明和实际源码验证结果。
