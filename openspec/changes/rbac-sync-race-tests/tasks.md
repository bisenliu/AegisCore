## 1. 现状梳理与测试切入点

- [x] 1.1 梳理 `permission/infrastructure/redis` watcher 的 fake、状态快照、Stop 和重订阅测试入口，确认是否需要最小测试 helper。
- [x] 1.2 梳理 `permission/infrastructure/casbin` enforcer 的 loader/reload fake、waiter、root cancel 和 force refresh 测试入口，确认是否需要最小测试 helper。

## 2. watcher race/stress 测试

- [x] 2.1 增加 watcher 并发 Pub/Sub hint、周期 revision check、cache invalidation 与 revision-aware reload 交错测试。
- [x] 2.2 增加 Redis 断连、message channel 关闭、重订阅恢复期间状态语义测试，断言 reconnecting 时 lifecycle 仍为 running。
- [x] 2.3 增加 watcher `Stop` 与阻塞 revision source、阻塞 reload engine、订阅/Receive/退避并发的测试，断言 Stop 后 lifecycle stopped、subscription stopped，正常 cancellation 不记录为业务 failure。
- [x] 2.4 增加 Stop 超时和重复 Stop 安全测试，确保 fake 清理不会泄漏 goroutine。

## 3. enforcer race/stress 测试

- [x] 3.1 增加多个 waiter 并发等待重复、乱序和递增 target revision 的测试，断言 applied revision 不倒退且未取消 waiter 正确完成。
- [x] 3.2 增加单个 waiter cancel 不取消共享 reload、leader cancel 与 engine root cancel 的 race 测试。
- [x] 3.3 增加强制刷新加入正在构造的普通 reload 的测试，断言 force refresh 到达后重新读取快照。

## 4. 测试目标与验证

- [x] 4.1 增加或更新带服务前缀的 permission sync race test 目标或说明，运行 `go test -race ./user-service/internal/features/permission/infrastructure/redis ./user-service/internal/features/permission/infrastructure/casbin`。
- [x] 4.2 运行相关普通测试与架构检查：`go test ./user-service/internal/features/permission/infrastructure/redis ./user-service/internal/features/permission/infrastructure/casbin`、`make user-service-architecture-lint`。
- [ ] 4.3 暂存本次预期代码、测试和 OpenSpec change 变更，排除 Multica runtime 文件。
- [ ] 4.4 运行 `make lint`。
- [ ] 4.5 运行 `make verify`，如因运行环境或非本次变更导致失败，记录具体原因和受影响命令。
