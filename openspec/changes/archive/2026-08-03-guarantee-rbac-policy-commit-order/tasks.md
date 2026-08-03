## 1. Revision 提交顺序

- [x] 1.1 新增固定单行 RBAC policy revision counter Ent schema，生成 Ent 代码和 Atlas migration，并以已有最大 revision 初始化 counter。
- [x] 1.2 调整 role PostgreSQL transaction helper，通过 counter 原子递增显式分配 revision，并覆盖 counter、revision、outbox 或 commit 失败时的完整回滚。
- [x] 1.3 增加 Docker-backed PostgreSQL 受控并发测试，证明较大 revision 不能先于较小 revision 提交，且 100 并发 mutation 的 revision/outbox 唯一并最终有序。

## 2. Projection 刷新协议

- [x] 2.1 扩展 permission application reload port 和 Casbin engine，实现可 coalesce 的强制当前快照刷新，相同 revision 强制候选可交换且较旧候选不可覆盖。
- [x] 2.2 调整 Redis watcher，使每条 `policy_changed` 都执行强制刷新，`user_role_changed` 在无 gap 时定向失效、跨 gap 时全量失效并追赶 revision。
- [x] 2.3 增加重复、乱序和相同 revision 强制刷新测试，并运行 permission 相关 package 的 `go test -race`。

## 3. API 提交语义

- [x] 3.1 调整在线 role command，使 transaction 提交后的本地同步失败只记录并保持 fail-closed，不再向 API 返回 mutation 失败。
- [x] 3.2 调整用户角色和角色权限 Add/Remove store，在同一 transaction 内返回最终集合并删除 command 的提交后响应查询。
- [x] 3.3 更新 command、PostgreSQL store、HTTP controller 和 mock 测试，覆盖提交前失败、提交后同步失败成功响应及最终集合。

## 4. 故障验收与文档

- [x] 4.1 增加真实 outbox dispatcher、Redis publisher、watcher 和至少两个 Casbin engine 的 Redis 故障恢复与重放测试，验证无需新写即可 lag 归零和授权收敛。
- [x] 4.2 更新 `docs/TESTING.md`，区分 unit harness 与 Docker-backed 端到端 RBAC 故障验收命令和断言。

## 5. 生成与验证

- [x] 5.1 运行 `openspec validate guarantee-rbac-policy-commit-order`、`make user-service-generate`、`make user-service-migrate-validate`、`make user-service-openapi-generate` 和生成物 drift 检查。
- [x] 5.2 运行相关 package 测试、Docker-backed RBAC 测试和 `make user-service-architecture-lint`。
- [x] 5.3 暂存本 change 的预期代码、规格、文档、migration 和生成物后运行 `make lint` 与 `make verify`，确认无未暂存 drift。
