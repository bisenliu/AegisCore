## Why

`user-service/cmd` 的根命令测试目前通过替换 package-level 可变函数变量注入替身，测试之间必须保存和恢复全局状态，阻碍并行化并增加竞态风险。需要把 `serve` 和 RBAC 子命令的测试替身改为命令构造时的局部依赖，让 CLI 装配保持可测试且不依赖全局可变 hook。

## What Changes

- 移除 `user-service/cmd/main.go` 中 `newLifecycleApp`、`runRBACSeed`、`runAssignSuperAdmin`、`runCreateSuperAdmin` 这类 package-level 可变函数变量。
- 将 `newRootCommand` 调整为接收显式命令依赖结构，生产入口使用默认依赖构造 root command。
- 将 `runServe` 需要的 lifecycle app factory 改为通过参数或命令依赖传入。
- 更新 `user-service/cmd/main_test.go`，让每个测试在本地构造替身依赖，不再赋值和恢复全局函数变量。
- 保持 `serve`、`rbac seed`、`rbac assign-super-admin`、`rbac create-super-admin` 的命令名称、flag、默认值、输出语义和退出码不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: user-service 根命令测试和 serve lifecycle 测试必须通过局部依赖注入表达替身，避免 package-level 可变函数变量造成并行化前提缺失。
- `rbac-access-control`: RBAC CLI command 测试必须通过根命令局部依赖注入覆盖 seed、assign-super-admin 和 create-super-admin runner，且不改变 RBAC 引导生产语义。

## Impact

- 代码：`user-service/cmd/main.go`、`user-service/cmd/main_test.go`。
- API/CLI：不改变 CLI 命令路径、flag、默认值、输出语义或退出码。
- 数据库/OpenAPI/部署资产：不涉及 Ent schema、Atlas migration、OpenAPI 生成物或部署清单。
- 验证：在 `user-service` 模块下运行 `go test -race ./cmd`，并运行 `make user-service-architecture-lint` 校验 OpenSpec 中文与结构约束。
