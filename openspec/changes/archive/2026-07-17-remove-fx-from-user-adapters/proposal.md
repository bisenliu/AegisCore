## Why

用户 feature 的 PostgreSQL adapter constructor 当前暴露 `fx.In`、`name:"primary_db"` 等 Fx/Dig metadata，使基础设施构造 API 与特定 DI framework 耦合，降低了普通 Go 装配、单元测试和架构边界检查的清晰度。

现在需要以不兼容方式收敛 user feature 的稳定构造约束，让用户资料调用链保持业务行为不变的同时，可由消费侧通过显式 port 和普通 Go 参数直接装配。

## What Changes

- **BREAKING** 移除 user feature 基础设施 constructor API 中的 `UserStoreParams`、`fx.In` 和 `name:"primary_db"` metadata。
- **BREAKING** 将 user PostgreSQL store constructor 改为接收显式 `*ent.Client` 参数，不保留旧 constructor 或兼容 wrapper。
- 更新生产 Fx composition 和测试调用点，使 Fx module 只在 composition 层适配新签名。
- 补充架构检查，禁止 user feature 的 domain、application、infrastructure 和 transport 生产包导入 Fx/Dig；保留 user feature 的 Fx module 本身。
- 修改 `user-identity-management` 规格，将稳定约束调整为 framework-neutral constructor 和消费侧 port 注入。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-identity-management`: 用户 feature 边界要求从可携带 Fx/Dig metadata 的基础设施构造 API，调整为 framework-neutral constructor 与消费侧 port 注入约束。

## Impact

- 影响代码：`user-service/internal/features/user` 下 PostgreSQL adapter constructor、Fx module 生产装配和相关测试。
- 影响架构检查：新增或更新检查规则，覆盖 user feature 生产包中对 `go.uber.org/fx`、`go.uber.org/dig`、`fx.In` 和 `fx.Out` 的禁止导入或使用约束，并排除允许承载 composition 的 `fx.go`、`fx_test.go`。
- 不影响用户 HTTP API、DTO、OpenAPI、Ent schema、Atlas migration、业务校验、授权行为、PostgreSQL/Ent 创建、Ping 或关闭所有权。
- 不删除 user feature 的 Fx module，不改变服务级 Fx runtime 组装模式。
