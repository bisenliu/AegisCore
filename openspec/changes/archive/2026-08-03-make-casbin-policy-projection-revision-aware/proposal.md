## Why

当前 Casbin reload 将“收到或处理过的通知 version”近似视为本地已应用版本，但 enforcer 构造与交换没有绑定数据库 policy revision；并发或乱序 reload 因而可能让旧快照覆盖新快照，也可能让 tracker、lag、readiness 显示的 revision 高于实际授权投影。需要让本地 Casbin enforcer 与其数据库 revision 作为一个不可分割的 revision-aware projection 应用，确保观测到的收敛状态等于真实授权状态。

## What Changes

- 扩展 policy loader 返回包含数据库 revision 与授权规则的 `PolicySet`，并保证面向目标 revision 的加载结果不低于该目标。
- 将无 revision 的 reload 契约替换为 `ReloadToRevision(ctx, targetRevision)` 或等价接口；engine 在候选 enforcer 完整构造后，只允许在锁内以不倒退 revision 的方式交换投影。
- 在 engine 内统一维护实际 applied revision、最近 reload error 和 reload status，并让 tracker、startup/readiness 与 lag 使用同一投影 revision 语义。
- 为同实例并发 reload 建立串行化或 coalesce 协议，合并目标 revision 并保证高并发写入后最终应用数据库 latest revision。
- 调整 watcher、coordinator 和相关 application port，使本地 reload、Pub/Sub 与周期补偿都以数据库 policy revision 为目标，不再把通知处理进度冒充已应用 revision。
- 增加可控乱序、100 并发目标、reload 失败、fail-closed、metrics 与 startup/readiness 测试。
- **BREAKING**：移除无 revision 的旧 reload 路径和通知序号作为 applied revision 的兼容语义。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：将 Casbin 本地授权投影、reload 防倒退、applied revision、lag 和健康状态统一绑定到实际数据库 policy revision。

## Impact

- Go 代码：影响 `user-service/internal/features/permission/` 的 policy loader、Casbin engine、reload coordinator、watcher、version tracker、application port、composition、metrics 与 health/status；不向 `common/` 或 `internal/shared/` 下沉 RBAC revision 语义。
- PostgreSQL：读取前置 change 提供的最新 policy revision 与关系授权快照；不新增或修改 revision/outbox schema，不实现 outbox dispatcher。
- Redis：继续作为数据库 revision 通知与补偿加速层；消息携带的 revision 只作为 reload target，不能直接推进 engine applied revision。
- Casbin 与安全：enforcer 交换必须防止 revision 倒退；加载或交换失败不得提升 applied revision，授权保持 fail-closed 或继续使用上一成功投影并暴露不可用状态。
- 可观测性：调整现有 policy reload、lag、tracker、startup/readiness 语义与测试，使 lag 为 0 必须表示本地实际投影不落后于已知数据库 revision；不新增高基数标签。
- API、OpenAPI、数据库 migration 与部署协议：HTTP 契约和 OpenAPI 不变，无新 migration；依赖 `add-rbac-policy-revision-outbox` 与 `add-rbac-policy-outbox-dispatcher` 提供数据库 revision 和 revision 通知。
- 规格与验证：新增 `rbac-access-control` delta，并运行相关单元、并发/race-sensitive、metrics/health 测试及 `make user-service-architecture-lint`、`make lint` 和 `make verify`。
