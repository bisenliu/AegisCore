## Why

`user-service/cmd/rbac.go` 目前通过 package-level 可变依赖工厂为 RBAC 命令装配运行时依赖，测试通过替换该全局变量注入替身。该做法让并行测试之间存在依赖工厂串扰和数据竞争风险，也让命令入口的依赖关系不够显式。

## What Changes

- 移除 `newRBACSeedDependencies` package-level 可变工厂，不保留兼容入口。
- 将 `runRBACSeedCommand`、`runAssignSuperAdminCommand`、`runCreateSuperAdminCommand` 的依赖工厂改为调用链显式传递。
- 调整 `user-service/cmd/main.go` 的 RBAC 命令组装，使生产依赖工厂在命令创建时注入。
- 调整 RBAC 命令测试 helper，使测试通过局部依赖或局部 runner 注入替身，不再写 package-level 状态。
- 保持 RBAC seed、超级管理员绑定、超级管理员创建、超时、配置加载、数据库连接和 Ent client 清理顺序不变。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：明确 RBAC CLI 引导命令的依赖装配必须使用单次命令调用范围内的显式依赖，不得依赖可变 package-level 工厂注入运行时依赖。

## Impact

- 影响代码：`user-service/cmd/rbac.go`、`user-service/cmd/rbac_test.go`，以及必要的 `user-service/cmd/main.go` 调用点。
- 不影响 HTTP API、OpenAPI、数据库 schema、Atlas migration、部署资产或外部依赖。
- 不改变命令行输出文本和 RBAC 业务行为，仅收敛命令依赖装配方式与测试隔离边界。
- 验证重点为 `user-service` 模块下 RBAC command 相关单元测试和 race 检测。
