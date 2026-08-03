## 1. Revision-aware policy snapshot

- [x] 1.1 扩展 `user-service/internal/features/permission/infrastructure/casbin/` 的 `PolicySet` 与 loader port，使加载结果包含数据库 `Revision`，并删除无 target 的 `LoadPolicies` 兼容入口。
- [x] 1.2 使用现有 Ent/PostgreSQL driver 实现只读 `REPEATABLE READ` snapshot：在同一 transaction 中读取 latest policy revision、角色权限规则和固定超级管理员 wildcard，确保返回 revision 与规则一致且不低于调用方 target。
- [x] 1.3 实现 target 尚不可见时结束旧 snapshot、按 context deadline 有界重试的行为，覆盖 revision 0 空库、context 取消、revision/rule 查询和 transaction 失败，不新增 schema、migration 或跨 feature persistence 抽象。
- [x] 1.4 补充 loader 单元与 PostgreSQL 集成测试，证明 mutation/revision 原子提交后快照绑定、低 revision 不返回、重试会打开新 snapshot，并运行对应 permission Casbin package 测试。

## 2. Engine 投影状态与并发协议

- [x] 2.1 将 `Engine.Reload(ctx)` 替换为 revision-aware `ReloadToRevision(ctx, targetRevision)` 或等价 application port，统一保存 enforcer、applied revision、last error 和 reload status，并暴露不可独立写入的状态 snapshot。
- [x] 2.2 实现候选 enforcer 锁外构造和锁内 revision 比较/原子交换：更高 revision 才替换，相等幂等成功，stale候选丢弃；任何失败保留上一成功enforcer/revision且不得清除错误或提升revision。
- [x] 2.3 在engine内实现max-target single-flight/coalesce协议，使同一实例只有一个leader执行加载/构造，waiter按自身target等待，单个waiter取消不终止其他调用，且不引入常驻goroutine或通用runtime抽象。
- [x] 2.4 更新`Enforce`与authorization service的fail-closed检查，使engine未初始化、target未追平或最近reload失败时即使保留旧enforcer也拒绝授权，成功追平后才恢复。
- [x] 2.5 增加受控channel loader测试，覆盖revision 1晚于revision 2完成、equal/stale候选、加载/构造失败与恢复、waiter取消，并使用`go test -race`验证engine相关package。
- [x] 2.6 增加100并发target测试，证明最终applied revision等于加载时数据库latest且不低于最高target，同时验证coalesce不要求逐revision构造。

## 3. Coordinator、watcher 与唯一 applied 来源

- [x] 3.1 调整permission application的policy reload/status port与本实例coordinator，使role mutation返回的数据库revision直接作为target，并删除无revision调用路径。
- [x] 3.2 改造Redis watcher的Pub/Sub与周期补偿路径，以数据库revision调用engine；重复、相等和乱序消息保持幂等，`policy_changed`成功后执行全量cache invalidation，定向`user_role_changed`继续执行其side effect但不得推进Casbin applied revision。
- [x] 3.3 删除可由watcher独立`MarkApplied`的`VersionTracker`，或将其收窄为只读委托engine状态的facade；更新composition wiring，确保engine是applied revision唯一事实源且有状态组件仍只构造一次。
- [x] 3.4 更新coordinator/watcher单元测试，覆盖数据库revision贯穿、消息处理成功但engine未应用时revision不推进、重复/乱序通知、周期补偿和cache invalidation错误语义。

## 4. 状态、观测与健康门禁

- [x] 4.1 更新policy reload metrics与状态采集，使applied值来自engine实际投影、失败不提升gauge，并保持现有低基数label和日志安全字段约束。
- [x] 4.2 将reload lag统一为`max(known_latest_database_revision - engine_applied_revision, 0)`，确保消息接收、Redis max更新或reload失败不会清零lag，且无独立tracker数值参与计算。
- [x] 4.3 更新startup/readiness和router health接线，使未初始化、最近reload失败或applied低于已知target时报告policy不可用；恢复成功后状态、last error与revision原子一致。
- [x] 4.4 补充metrics、authorization、startup/readiness和router health测试，证明`tracker/applied`与engine实际投影一致、lag为0表示投影不落后、失败期间授权与业务流量均fail-closed。

## 5. 规格与交付验证

- [x] 5.1 对照`openspec/changes/make-casbin-policy-projection-revision-aware/specs/rbac-access-control/spec.md`检查实现，确认未创建revision/outbox schema、未实现dispatcher、未处理user-role cache inflight回填且未保留无revision reload兼容路径。
- [x] 5.2 运行permission/role相关精确Go package测试和race-sensitive测试，记录命令并修复全部失败；确认100并发与受控乱序验收场景稳定通过。
- [x] 5.3 运行`make user-service-architecture-lint`，确认RBAC revision语义留在permission feature，未泄漏到`common/`、`internal/shared/`、`internal/integration/`或application/domain concrete persistence依赖。
- [x] 5.4 检查`git diff`，确认无Ent生成物、Atlas migration、OpenAPI、deployments/dashboard或其他非预期diff；如意外产生生成物，使用对应生成/check命令验证并仅保留需求内产物。
- [x] 5.5 在全部实现、测试、规格和文档完成后，仅暂存本change的预期代码与OpenSpec artifacts，检查`git status`和staged diff不包含无关或敏感文件。
- [x] 5.6 在预期变更已暂存后运行`make lint`；只有命令通过后才完成本任务。
- [x] 5.7 在预期变更已暂存后运行`make verify`；只有全部验证和最终drift检查通过后才将change标记为实现完成。
