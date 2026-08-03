## Context

当前 RBAC 授权以 PostgreSQL 关系数据作为业务权威来源，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。在线角色状态、角色权限和用户角色绑定写入成功后，系统需要刷新本实例并通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本。

这类同步链路存在多种 P0 风险：数据库提交后 Redis 暂时不可用可能导致其他副本长期落后；并发 reload 可能乱序完成并覆盖较新的 projection；Add/Remove/Replace 写操作重放可能暴露非幂等行为；高并发写可能造成 applied revision 假收敛；用户角色 cache 延迟或未失效可能让授权结果偏离数据库权威状态。本 change 只建立验证套件和文档，不改变生产 runtime 行为。

受影响路径预计集中在 `user-service/internal/features/permission/`、`user-service/internal/features/role/` 或现有 integration 测试目录。测试 harness 可以放在对应 `_test.go` 或测试专用 helper 中，不放入 `common/`、`user-service/internal/shared/`、生产 `internal/integration/` 或正式 runtime API。

## Goals / Non-Goals

**Goals:**

- 建立 RBAC policy sync 故障注入验收套件，覆盖数据库 revision、同步通知、dispatcher、watcher、Casbin projection 与用户角色 cache 的端到端收敛。
- 使用 channel、barrier、fake clock/eventually-style 条件和明确 deadline 控制并发时序，避免固定 `time.Sleep` 作为状态已变化的主要判断依据。
- 覆盖 Redis publish 失败后恢复、无新增写入的副本 lag 归零场景。
- 覆盖两次 reload 逆序完成时，最终 Casbin projection 仍对应最新 revision 的场景。
- 覆盖 Add/Remove/Replace outbox 重放不丢通知、不产生非幂等破坏的场景。
- 覆盖 100 并发 RBAC 写及最终 applied revision 收敛的场景。
- 更新测试说明，明确每个故障注入场景的风险、运行命令、容器依赖开关和预期收敛条件。

**Non-Goals:**

- 不实现业务修复逻辑。
- 不引入新的同步协议、数据库 schema、migration、Redis key schema 或生产配置。
- 不改变 HTTP API、OpenAPI、部署清单、观测资产或生产授权语义。
- 不新增跨 feature testing facade、全局可变 hook、`ForTest` 正式 API 或测试专用生产 adapter。
- 不通过放宽 fail-closed、忽略同步错误或跳过 notifier 来使测试通过。

## Decisions

### Decision: 测试 harness 留在消费 feature 或 integration 测试边界

测试辅助类型和故障注入控制点 SHALL 位于 `permission`/`role` 相关测试包或现有 user-service integration/e2e 测试边界中，只通过现有 constructor、port 和真实组件组合进行注入。

选择理由：policy sync 是 user-service 内业务能力，测试需要贴近消费侧端口和现有 runtime 组合；放入 `common` 会泄漏 RBAC 业务语义，放入 `internal/shared` 会违反 shared 只承载稳定业务内核的边界。

备选方案：新增公共测试框架或生产级 fault injection adapter。该方案会扩大 API 面并引入与业务无关的正式代码，因此不采用。

### Decision: 使用确定性并发控制替代固定 Sleep

故障注入套件 SHALL 使用 channel、barrier、手动阻塞 loader、受控 fake notifier、eventually 条件和 test deadline 控制时序。允许短 tick 轮询等待条件，但等待结果必须绑定状态谓词和 deadline。

选择理由：RBAC 同步问题来自提交、通知、reload 和缓存失效的相对时序，固定 `time.Sleep` 容易在 CI 中产生 flake 或假通过。

备选方案：用大 sleep 等待所有 goroutine 自然完成。该方案无法证明具体 interleaving，且会拖慢 CI，因此不采用。

### Decision: 优先组合真实 store/loader/engine 与可控边界替身

涉及 PostgreSQL revision、关系数据、Casbin projection 和用户角色 cache 的断言 SHALL 尽量使用真实实现。Redis publish、watcher 消息、dispatcher 重试和 loader/cache 延迟可以用测试替身控制故障、调用顺序和重放。

选择理由：真实 store/loader/engine 能验证最终授权投影，不把测试降级为 mock 交互检查；可控边界替身能稳定制造 Redis 故障、乱序和重放。

备选方案：全量 e2e 只使用真实 Redis 并通过网络扰动制造故障。该方案更接近生产但难以稳定复现精确乱序，不适合作为每次 CI 的主要验收。

### Decision: 收敛断言以 revision 和授权结果双重确认

测试 SHALL 同时断言 applied revision 或副本 lag 归零，以及关键用户/角色/权限组合的 Casbin 授权结果与数据库权威状态一致。仅看到 watcher 收到消息或 dispatcher 调用完成不足以视为收敛。

选择理由：历史风险包括假收敛，即同步指标或通知链路看似完成但本地 policy/cache 仍停留在旧状态。

备选方案：只断言通知发送或 reload 被调用。该方案不能覆盖 projection 偏移，因此不采用。

### Decision: 容器依赖遵循现有测试开关

需要真实 PostgreSQL/Redis 的测试 SHALL 复用 `common/testing/containers/`，并遵循 `AEGISCORE_TEST_CONTAINERS=1`。未启用容器开关时，相关测试可以按仓库现有约定跳过；启用后 Docker、镜像、容器或 migration 失败 MUST 使测试失败。

选择理由：保持 CI 与现有测试门禁一致，不新增 `TEST_CONTAINERS` 或其他兼容别名。

备选方案：引入独立环境变量或外部手工 Redis/PostgreSQL。该方案会增加 CI 复杂度并削弱可重复性，因此不采用。

## Risks / Trade-offs

- [Risk] 故障注入套件触发真实竞态后在 CI 中 flake → Mitigation: 所有并发点使用 barrier、事件通道和 deadline，失败时输出当前 revision、applied revision、lag、消息序列和授权结果。
- [Risk] 为了测试便利引入生产 hook 或共享测试 facade → Mitigation: architecture lint 已禁止 `ForTest`/`testHook` 正式 symbol；本 change 只在 `_test.go` 或测试 helper 中放置 harness。
- [Risk] 全量容器集成测试耗时增加 → Mitigation: 将慢速场景集中在容器开关下，package-level 可控替身测试覆盖不依赖外部容器的乱序与重放逻辑。
- [Risk] 前置业务修复未完成时新增验收测试失败 → Mitigation: 测试名称和文档明确其 P0 复现意图；在对应修复 change 完成前可作为红灯验收基线，不通过放宽断言规避失败。
- [Risk] 只验证 revision 不验证授权投影 → Mitigation: 每个收敛场景必须包含至少一个授权 allow/deny 或用户有效权限断言。

## Migration Plan

- 本 change 不包含数据库 migration、OpenAPI 生成、部署发布或生产配置迁移。
- 实施时先新增测试 harness 和失败场景，再补充文档和 spec delta 验证。
- 回滚策略为移除本 change 新增的测试和文档/spec delta；不会影响运行时数据或生产行为。

## Validation

- 运行相关 package 测试，优先覆盖 `permission`、`role` 和新增故障注入测试所在包。
- 启用容器依赖后运行 `AEGISCORE_TEST_CONTAINERS=1 make test` 或更窄的等价包测试命令。
- 文档或规格变更后运行 `openspec validate --specs` 与 `make user-service-architecture-lint`。
- 合并前按仓库要求运行 `make lint` 和 `make verify`。

## Open Questions

- 最终测试文件的精确放置路径需在实现时根据现有 policy sync 组件和测试包布局确定。
- 若前置修复尚未落地，部分验收测试是否先以明确失败的红灯测试提交，需由实施阶段结合当前主干状态判断。
