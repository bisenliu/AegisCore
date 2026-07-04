## Why

`user-service rbac` 的 seed、assign-super-admin 和 create-super-admin 命令是 RBAC 初始化和超级管理员引导入口，但当前命令 runner、依赖装配和参数归一化路径缺少直接测试覆盖。补齐这些测试可以把现有 CLI 契约、配置来源、错误传播和 cleanup 语义固定下来，避免后续通过旧命令名、旧环境变量或旧 bootstrap 行为引入兼容分支。

## What Changes

- 为 RBAC CLI 命令 runner 补齐单元测试，覆盖 seed、assign-super-admin、create-super-admin 的成功路径、参数缺失、配置错误、依赖初始化错误和 cleanup 错误合并语义。
- 为 `createSuperAdmin` 和 `normalizeCreateSuperAdminOptions` 补齐测试，覆盖已有用户、新建用户、重置密码、环境变量归一化和必要输入校验。
- 在命令测试中优先使用可注入依赖或最小 fake dependency 验证流程，避免为测试启动真实外部 PostgreSQL；如必须使用 Ent test client，需要保持隔离。
- 测试断言遵循 `docs/TESTING.md` 和 `delivery-operations` 的语义化断言规范，错误、缺失参数、cleanup 合并和依赖初始化失败使用 `require` fail-fast 语义。
- **BREAKING**：测试只固化当前 `user-service rbac` 命令契约和当前配置来源，不新增旧 CLI alias、旧 flag、旧环境变量、旧 root Makefile 无服务前缀入口或旧 bootstrap 兼容路径。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`：补充 RBAC CLI seed、超级管理员绑定和超级管理员创建命令的测试覆盖要求，固定当前命令契约和引导语义。
- `delivery-operations`：补充命令测试断言规范，要求 CLI 错误、参数缺失、依赖初始化和 cleanup 语义使用语义化 fail-fast 断言。

## Impact

- 影响代码：`user-service/cmd/rbac.go`、`user-service/cmd/main.go` 及同包测试；生产代码只允许为现有 CLI 流程提供合理可测试性 seam。
- 影响规格：新增 `openspec/changes/cover-rbac-cli-commands-no-compat/specs/rbac-access-control/spec.md` 和 `openspec/changes/cover-rbac-cli-commands-no-compat/specs/delivery-operations/spec.md`。
- 不影响 HTTP API、OpenAPI、数据库 schema、Atlas migration、RBAC seed 业务语义、超级管理员默认绑定、role/permission baseline、Docker/Compose/Kubernetes/Helm 发布流程或生产配置文件格式。
