## 1. 基线扫描

- [x] 1.1 扫描 `user-service/internal/features/role/**/*_test.go` 和 `user-service/internal/shared/rbacbaseline/**/*_test.go` 中 `t.Fatalf`、`t.Fatal`、`t.Errorf`、`t.Error` 和 `Fail` 类调用，按 package 记录迁移范围。
- [x] 1.2 扫描目标范围内现有 `github.com/stretchr/testify/require` 和 `github.com/stretchr/testify/assert` 使用点，确认需要新增或调整 import 的文件。
- [x] 1.3 确认本次只修改 role、rbacbaseline `_test.go` 和本 change artifacts，不修改 role 生产代码、RBAC seed 语义、超级管理员绑定、PostgreSQL schema、HTTP API、OpenAPI 或部署资产。

## 2. 基础与边界测试迁移

- [x] 2.1 将 `domain`、feature-level 配置、HTTP input 和 shared RBAC baseline catalog 等低风险测试迁移为 `require` 语义化断言。
- [x] 2.2 将 `transport/http` controller、mapper、routes 和 response 相关测试迁移为 `require`，对多字段响应中互相独立字段失败收集按需使用 `assert`。
- [x] 2.3 保持 HTTP boundary 测试的路由、认证上下文、RBAC 授权检查和 envelope 校验语义不变，不新增旧 role 字段或旧 binding 兼容断言。

## 3. Application 与 seed 测试迁移

- [x] 3.1 将 `application/command` 测试中的角色创建、更新、启停、角色权限绑定和用户角色绑定断言迁移为 `require` 或必要的 `assert`。
- [x] 3.2 将 `application/query` 测试中的角色查询、列表、权限摘要和用户角色查询断言迁移为 `require` 或必要的 `assert`。
- [x] 3.3 将 `application/seed` 测试中的 RBAC baseline seed、超级管理员绑定和重复执行幂等断言迁移为 `require` 或必要的 `assert`。
- [x] 3.4 保持已有 gomock 生成物、matcher、expectation 和失败注入方式不变，不回退为手写 store/notifier double。

## 4. Infrastructure 测试迁移

- [x] 4.1 将 `infrastructure/postgres` 下 RoleStore 测试迁移为 `require` 语义化断言，不修改 Ent schema、migration 或查询生产逻辑。
- [x] 4.2 将 `infrastructure/postgres` 下 UserRoleStore 测试迁移为 `require` 或必要的 `assert`，保持用户角色绑定语义不变。
- [x] 4.3 将 `infrastructure/postgres` 下 RolePermissionStore 测试迁移为 `require` 或必要的 `assert`，保持角色权限绑定语义不变。

## 5. 格式化与残留例外

- [x] 5.1 对修改过的 Go 测试文件运行 `gofmt`，清理未使用 import 并保持生成 mock 文件使用方式不变。
- [x] 5.2 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/role user-service/internal/shared/rbacbaseline --glob '*_test.go'`，确认剩余命中均符合 `docs/TESTING.md` 特殊例外。
- [x] 5.3 在实现记录中列明 5.2 的剩余命中和保留原因；如果没有剩余命中，记录为无残留例外。
- [x] 5.4 运行 `rg "github.com/stretchr/testify/(require|assert)" user-service/internal/features/role user-service/internal/shared/rbacbaseline --glob '*_test.go'`，确认迁移后的实际使用点可定位。

## 6. 验证

- [x] 6.1 运行 `go test ./user-service/internal/features/role/... ./user-service/internal/shared/rbacbaseline/...` 并确认通过。
- [x] 6.2 运行 `openspec validate standardize-role-baseline-test-assertions-no-compat` 并确认通过。
- [x] 6.3 运行 `make user-service-architecture-lint`，确认 OPSX 文档语言、生成物 drift 和架构边界检查通过。
- [x] 6.4 将本次预期代码、测试和 OpenSpec 变更加到暂存区后运行 `make lint`，确认 lint 通过且不被预期 diff 影响。
- [x] 6.5 保持本次预期变更已暂存，运行 `make verify`，确认完整验证通过或记录无法运行的具体原因。

## 7. 剩余例外记录

- [x] 7.1 汇总最终残留例外；若最终扫描未发现 `t.Fatal`、`t.Error` 或 `Fail` 命中，记录为无残留例外。
