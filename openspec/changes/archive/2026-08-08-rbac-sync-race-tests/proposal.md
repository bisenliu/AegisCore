## Why

RBAC policy sync 链路同时涉及 Redis Pub/Sub、周期性数据库校准、Casbin engine reload、用户角色缓存失效和 Fx lifecycle，后续重构很容易引入竞态、状态倒退或关闭泄漏。当前规格已经定义核心收敛语义，但缺少针对 watcher/enforcer 并发 reload、Stop 竞态和异常恢复状态的 race/stress 测试门禁，无法稳定拦截这类回归。

## What Changes

- 为 RBAC watcher 与 Casbin enforcer 增加 race/stress 类测试，覆盖多条 Pub/Sub hint、周期 revision check、`RefreshToRevision`、`ReloadToRevision`、强制刷新和缓存失效交错。
- 增加 watcher `Stop` 与阻塞 revision source、阻塞 reload engine、关闭 message channel、Redis 断连重订阅之间的竞态测试。
- 增加 enforcer lifecycle root cancel、leader cancel、多个 waiter 并发等待和 force refresh 合并语义测试。
- 固化 watcher 结构化状态断言：Redis reconnecting 时生命周期仍为 running，Stop 完成后为 stopped 且 subscription stopped，reconcile cancellation 不记为业务 failure。
- 增加限定在 permission sync 相关包的 `go test -race` 可运行目标说明或 Makefile 子目标。
- 不修改公开业务接口、数据库 schema、OpenAPI、部署资产或生产同步行为；如需 hook，仅添加测试注入所需的最小 fake 或内部辅助。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`: 补充 watcher/enforcer 并发同步、关闭竞态和异常恢复状态语义的测试门禁要求。

## Impact

- 影响代码主要位于 `user-service/internal/features/permission/infrastructure/redis` 与 `user-service/internal/features/permission/infrastructure/casbin` 相关测试和必要的内部测试辅助。
- 可能影响仓库级或服务级 Makefile 中 permission sync race test 目标，但不新增无服务上下文的根目标。
- 不影响 HTTP API、OpenAPI、Ent schema、Atlas migration、Redis/PostgreSQL 生产协议、Casbin policy 数据模型或部署观测资产。
