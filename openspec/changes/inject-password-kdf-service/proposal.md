## Why

`common/security/password` 当前使用包级 `argon2Gate` 和 `argon2Queue` 作为进程内 Argon2id 资源门控，调用方只能接受统一默认并发和队列预算。微服务拆分后，不同服务、CLI 和测试需要按自身资源模型显式配置密码 KDF 预算，避免共享安全 primitive 隐含服务级运行时策略。

## What Changes

- **BREAKING**：移除 `common/security/password` 的包级 `HashContext`、`VerifyContext` 以及包级 Argon2 门控入口，不保留兼容 wrapper。
- 在 `common/security/password` 中引入可实例化的密码 KDF 服务，由调用方通过构造参数显式声明 Argon2id 并发上限和队列上限。
- user-service 在 provider/config 边界创建并注入密码 KDF 服务，认证登录、强制改密、用户创建和 RBAC CLI 均改为使用注入实例。
- 单元测试改为创建独立密码 KDF 服务实例，不再修改包级 channel。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`：`common/security/password` 的密码哈希与校验 primitive 改为显式实例化、显式资源预算、无包级全局门控的共享安全能力。

## Impact

- 影响 Go 代码：`common/security/password/`、user-service auth/user application、RBAC CLI、Fx provider/config 装配和相关测试。
- 影响安全边界：密码 KDF 仍使用 Argon2id 固定算法和哈希格式，但资源预算从包级默认变为服务显式配置，避免共享 primitive 隐含服务运行时假设。
- 影响共享契约：这是 `common/security/password` 导出 API 的破坏式变更，所有调用方必须迁移到实例方法。
- 不影响 HTTP API、OpenAPI 文档、数据库 schema、Ent migration、Redis key、Casbin policy、部署清单或观测资产。
