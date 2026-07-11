## 1. RBAC CLI 依赖装配

- [x] 1.1 调整 `user-service/cmd/rbac.go`，移除 `newRBACSeedDependencies` package-level 可变变量，并以显式 factory 参数连接 RBAC seed、assign-super-admin 和 create-super-admin runner。
- [x] 1.2 调整 `user-service/cmd/main.go` 的 RBAC 命令组装调用点，确保生产路径传入真实依赖工厂且命令输出、超时、配置加载和 cleanup 顺序不变。

## 2. 测试隔离

- [x] 2.1 调整 `user-service/cmd/rbac_test.go` 的 `withRBACSeedDependencyFactory` 或等价 helper，使测试通过局部 runner、局部命令对象或局部依赖参数注入替身，不再写 package-level 状态。
- [x] 2.2 覆盖 RBAC seed、assign-super-admin、create-super-admin 和 cleanup 链相关测试，确认局部替身下测试独立运行。

## 3. 验证

- [x] 3.1 运行 `go test -race ./cmd -run 'TestRBAC|TestCreateSuperAdmin|TestAssignSuperAdmin|TestNormalize|TestChainCleanup'`（在 `user-service` 模块下）。
- [x] 3.2 运行 `openspec validate remove-rbac-command-global-factory --strict` 和 `make user-service-architecture-lint`。
- [x] 3.3 将本次预期代码和 OpenSpec 变更暂存后运行 `make lint` 和 `make verify`，排除或说明 Multica runtime 文件导致的工作区脏状态影响。
