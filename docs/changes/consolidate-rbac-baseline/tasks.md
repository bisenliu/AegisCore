# Tasks

- [x] 新增 `permission/application/rbacbaseline`，合并系统角色、系统权限和默认绑定基线。
- [x] 删除 `permission/application/catalog` 与 `role/application/catalog` 旧入口。
- [x] 更新 role seed service 使用 `rbacbaseline`。
- [x] 更新 Casbin policy loader 使用 `rbacbaseline.SuperAdminRoleID`，移除重复硬编码。
- [x] 更新相关单元测试引用和 baseline 校验测试。
- [x] 更新 `AGENTS.md` 与 `docs/ARCHITECTURE.md` 的 RBAC baseline owner 说明。
- [x] 运行聚焦测试和旧引用扫描。
