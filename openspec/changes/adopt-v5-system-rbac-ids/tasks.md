## 1. 基线梳理

- [x] 1.1 检查 `user-service/internal/shared/rbacbaseline/` 中现有角色、权限、默认绑定和当前内联 UUID 字符串，整理需要固化为常量的 semantic name 清单
- [x] 1.2 检查 `user-service/internal/features/role/application/bootstrap/` 中 bootstrap 用户 ID 定义和引用，确认替换为 `rbacbaseline.BootstrapSuperAdminUserID` 的调用点
- [x] 1.3 检查文档、测试和脚本中对 `00000000-0000-0000-0000-000000000001`、`00000000-0000-0000-0000-000000000002` 或 permission UUID 的引用，记录需要同步更新的位置

## 2. 系统 ID 常量实现

- [x] 2.1 新增或整理 `user-service/internal/shared/rbacbaseline/ids.go`，定义 `SystemIDNamespace`、`SuperAdminRoleID`、`BootstrapSuperAdminUserID` 和所有 baseline permission ID 常量
- [x] 2.2 为每个系统 ID 常量添加中文注释，说明该常量由 `UUIDv5(SystemIDNamespace, "<semantic-name>")` 生成后固化
- [x] 2.3 确保 semantic name 只使用稳定业务语义，例如 `role:super-admin`、`user:bootstrap-super-admin`、`permission:<resource>:<action>`，不使用项目名、HTTP path、中文文案或 Go symbol
- [x] 2.4 保持 `common/runtime/id.NewUUID()` 逻辑不变，不在 common 中加入 user-service RBAC semantic key schema

## 3. RBAC 基线引用改造

- [x] 3.1 修改 `DefaultRoles()`，确保系统角色 ID 引用 `rbacbaseline.SuperAdminRoleID` 常量
- [x] 3.2 修改 `DefaultPermissions()`，确保所有 `PermissionSpec.PermissionID` 引用 permission ID 常量，不再内联 UUID 字符串
- [x] 3.3 修改 `DefaultRolePermissions()`，确保所有角色和权限绑定引用 `rbacbaseline` 常量并保持默认绑定语义不变
- [x] 3.4 检查 RBAC seed 代码，确保 seed 写入系统基线时只消费 baseline 常量，不调用 `id.NewUUID()` 或运行时 UUID v5 生成逻辑

## 4. Bootstrap 引用改造

- [x] 4.1 删除 bootstrap application package 内私有的 `BootstrapSuperAdminUserID` 常量或等价定义
- [x] 4.2 修改 bootstrap application，使固定 bootstrap 用户 ID 由 `rbacbaseline.BootstrapSuperAdminUserID` 解析并传入 store
- [x] 4.3 更新 bootstrap 相关单元测试和错误路径测试，使断言引用新的统一常量并保持一次性 bootstrap 语义不变

## 5. ID 一致性测试

- [x] 5.1 新增 `user-service/internal/shared/rbacbaseline/ids_test.go`，校验 `SystemIDNamespace` 和所有系统 ID 常量均可解析
- [x] 5.2 在测试中校验每个系统 ID 常量的 UUID 版本为 5，且等于 `uuid.NewSHA1(namespace, []byte(semanticName))` 的结果
- [x] 5.3 在测试中校验全部系统 ID 无重复
- [x] 5.4 在测试中校验 `DefaultPermissions()` 返回的 permission ID 全部来自已登记常量且无重复
- [x] 5.5 在测试中校验 `DefaultRolePermissions()` 引用的 role ID 和 permission ID 均存在于基线常量集合

## 6. 文档和规格同步

- [x] 6.1 更新相关 README 或 docs，说明普通业务 ID 使用 UUID v7，系统内置 RBAC/bootstrap/permission ID 使用 UUID v5 生成后固化常量
- [x] 6.2 更新项目初始化和项目重命名相关文档，明确新项目初始化可以生成新 namespace，已有项目重命名默认不得重算系统 ID
- [x] 6.3 确认本 change 不新增 Ent schema、SQL migration、OpenAPI 生成物、部署清单或观测资产变更

## 7. 验证

- [x] 7.1 运行 `make user-service-architecture-lint` 并修复发现的架构边界问题
- [x] 7.2 运行相关 Go 测试，至少覆盖 `cd user-service && go test ./internal/shared/rbacbaseline/... ./internal/features/role/... ./internal/features/permission/...`
- [x] 7.3 如 API 注解或 OpenAPI 生成物未变化，确认无需运行 `make user-service-openapi-generate`；如发生变化则运行并检查 diff
- [x] 7.4 将本次预期代码、文档和 OpenSpec 变更加到暂存区
- [x] 7.5 运行 `make lint` 并修复失败项
- [x] 7.6 运行 `make verify` 并确保最终 drift 检查通过
