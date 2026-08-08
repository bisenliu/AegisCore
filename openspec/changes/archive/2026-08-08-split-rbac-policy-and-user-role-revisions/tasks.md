## 1. 数据模型与生成物

- [x] 1.1 调整 Ent schema，新增或改造 user-role revision 持久化结构，并让 `rbac_policy_revision` 只记录会改变 Casbin 静态 policy 的事实
- [x] 1.2 调整 `rbac_policy_outbox_events` schema，使 `policy_changed` 与 `user_role_changed` 使用明确分离的 revision 字段、外键和幂等键语义
- [x] 1.3 运行 `make user-service-generate` 更新 Ent 生成代码，并检查没有手写修改 `user-service/internal/persistence/ent/` 生成物
- [x] 1.4 运行 `make user-service-migrate-diff name=split-rbac-policy-and-user-role-revisions` 生成 Atlas SQL migration
- [x] 1.5 运行 `make user-service-migrate-validate` 验证 migration，并检查 schema、migration 与生成物 diff

## 2. Role 写入与 outbox

- [x] 2.1 拆分 role PostgreSQL transaction helper，使角色状态和角色权限变更创建 policy revision，用户角色绑定变更创建 user-role revision
- [x] 2.2 更新用户角色 Add、Replace、Remove store 返回的 revision 语义，确保纯用户角色绑定不会写入 `rbac_policy_revision`
- [x] 2.3 更新 outbox event 创建逻辑，分别写入 `policy_changed` 和 `user_role_changed` 的 kind、revision、user ID、role ID 和 idempotency key
- [x] 2.4 更新 role application command service 的通知端口调用，明确区分 policy reload 通知和 user-role cache invalidation 通知
- [x] 2.5 补充 role PostgreSQL 集成测试，验证用户角色绑定只推进 user-role revision 和 outbox，不创建 policy revision；角色权限与角色状态仍推进 policy revision

## 3. Permission application 与 Casbin 边界

- [x] 3.1 重构 permission application 同步端口，移除以同一 revision 处理所有 `PolicyChange` 的路径，不保留旧单 revision 兼容分支
- [x] 3.2 实现 policy change 路径：调用 `ReloadToRevision` 或 `RefreshToRevision`，验证 projection status，并在成功后执行必要用户角色缓存失效
- [x] 3.3 实现 user-role change 路径：只执行指定用户角色缓存失效，缺少精确用户时执行全量用户角色缓存失效，禁止调用 Casbin policy loader 或推进 engine applied revision
- [x] 3.4 更新 Casbin engine 和 policy sync 单元测试，验证纯 user-role 变更下 `LoadPoliciesAtLeast`、`ReloadToRevision`、`RefreshToRevision` 调用次数均为 0
- [x] 3.5 保持 fail-closed 测试覆盖：policy reload 失败、policy 未 ready、用户角色回源失败和缓存失效并发时不得使用旧角色集合放行

## 4. Redis payload、dispatcher 与 watcher

- [x] 4.1 更新 Redis policy refresh message envelope，`policy_changed` 只接受 `policy_revision`，`user_role_changed` 只接受 `user_role_revision` 和 `user_id`，删除旧 payload 兼容解析
- [x] 4.2 更新 publisher 和 dispatcher 测试，验证至少一次投递下重复事件保持幂等，Ack 失败重投不会导致已应用 policy revision 重复 reload
- [x] 4.3 重构 watcher 主循环，在收到消息后 drain 当前可用消息形成 bounded batch，并聚合最高 policy revision、user ID 集合和全量缓存失效标记
- [x] 4.4 更新 watcher policy 处理：只有最高未应用 policy revision 或 projection 不 ready 时才执行一次 reload；重复、相等或乱序已应用 policy 通知跳过全量 reload
- [x] 4.5 更新 watcher user-role 处理：始终执行消息要求的用户角色缓存失效，存在 revision gap、缺少 user ID 或无法证明精确集合完整时失效全部用户角色缓存，不推进 Casbin applied revision
- [x] 4.6 补充 watcher 负载测试，验证 100 条重复或连续 policy 通知下 loader 调用次数有常数上界，100 条纯 user-role 通知下 policy loader 调用次数为 0

## 5. 观测、部署与文档

- [x] 5.1 更新 RBAC metrics、日志字段和 status，使 policy revision 与 user-role revision 字段语义分离，metrics label 仍保持低基数
- [x] 5.2 更新 Prometheus alert、Grafana dashboard 源、Compose provisioning 副本、fixture 和 runbook 文案，确保 policy reload lag 只表示 latest policy revision 与 local applied policy revision 的差值
- [x] 5.3 运行 `make compose-dashboard-check` 或对应 dashboard 生成/检查脚本，确认观测资产无 drift
- [x] 5.4 更新 `docs/ARCHITECTURE.md`、`docs/TESTING.md` 或相关 OPSX 文档中涉及 RBAC revision/outbox/watcher 的说明
- [x] 5.5 运行 `make user-service-architecture-lint` 验证架构文档和边界规则

## 6. 回归验证与收尾

- [x] 6.1 运行 permission 和 role 相关包测试，覆盖 application、Redis watcher、Casbin engine、PostgreSQL store 和 dispatcher
- [x] 6.2 运行 `make user-service-test` 验证 user-service 模块回归
- [x] 6.3 运行 `openspec validate split-rbac-policy-and-user-role-revisions` 验证 change artifacts
- [x] 6.4 检查 `git diff`，确认只包含本 change 预期代码、migration、生成物、观测资产、文档和 OpenSpec artifacts
- [x] 6.5 将本次预期变更加到暂存区，再运行 `make lint`
- [x] 6.6 在本次预期变更已暂存后运行 `make verify`，确保最终 `git diff --exit-code` 不被未暂存预期变更阻塞
