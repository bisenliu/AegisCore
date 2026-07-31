## 1. 数据库 latest revision source

- [x] 1.1 在permission application定义只读最小latest policy revision query port及稳定错误边界，确认接口只表达数据库revision业务语义且不导入Ent、Redis或composition metadata。
- [x] 1.2 在permission infrastructure中使用现有Ent client实现latest revision查询，覆盖空`rbac_policy_revisions`表返回`0`、最大revision、context取消和数据库错误，不新增Ent schema、Atlas migration或outbox逻辑。
- [x] 1.3 在permission composition中注入现有named `primary_db`资源并把revision source提供给同一watcher实例，确认`common/`、`internal/shared/`和`internal/integration/`没有新增RBAC revision语义。
- [x] 1.4 补充revision source单元或PostgreSQL集成测试并运行对应permission infrastructure package测试，证明查询只读取已提交latest revision且错误保留cause。

## 2. watcher数据库revision补偿

- [x] 2.1 改造watcher周期性`CheckVersion`，每次从database revision source读取latest revision并与engine actual applied revision比较，删除Redis `CurrentVersion()`作为补偿权威值的调用和分支。
- [x] 2.2 改造Pub/Sub payload处理，使合法消息只作为唤醒hint并先校准database latest revision；保留hint revision、reason与既有cache side effect，禁止payload或Redis counter直接推进applied revision、清零lag或成为reload target。
- [x] 2.3 统一数据库revision mismatch到`ReloadToRevision(databaseLatest)`的控制流，确保engine未ready、reload未达到目标或失败时保持fail-closed且后续hint/周期检查可恢复，成功仅以actual applied不低于database latest判定。
- [x] 2.4 更新watcher lifecycle与composition相关测试，确认新增database source依赖不会重复构造engine/watcher、不会关闭共享Ent/Redis资源，`Start`/`Stop`幂等语义保持不变。

## 3. 故障与最终收敛测试

- [x] 3.1 增加Pub/Sub消息完全丢失测试，设置database latest高于local applied并保持Redis counter缺失或落后，证明周期检查触发reload且最终收敛。
- [x] 3.2 增加Redis重复、乱序和旧消息测试，证明每次处理以database latest校准，旧revision不得覆盖新projection或降低engine applied revision。
- [x] 3.3 增加Redis counter不存在、被重建和Redis故障恢复测试，证明这些状态不影响database revision补偿判断，Redis恢复后的旧hint不伪造收敛。
- [x] 3.4 增加database revision source不可用与恢复测试，证明失败记录稳定诊断、不使用Redis/payload target、不清零lag，恢复后下一次检查重新校准并reload。
- [x] 3.5 增加本地reload失败后恢复测试，证明失败不推进actual applied、不记录success且lag保持非零，后续database latest检查成功后actual applied追平并使lag为`0`。
- [x] 3.6 运行permission watcher、revision source和revision-aware engine相关package测试及必要的`go test -race`，确认消息并发或恢复路径没有projection倒退和数据竞争。

## 4. metrics与结构化日志

- [x] 4.1 更新permission Metrics接口、no-op生成物、mock和Prometheus recorder的固定allowlist，区分`revision_store_unavailable`、`revision_mismatch`、`reload_failed`与success，并删除仍表达Redis version store权威性的reason语义。
- [x] 4.2 将`aegiscore_user_service_rbac_policy_reload_lag`唯一计算改为`max(database_latest_policy_revision - local_applied_policy_revision, 0)`；只在成功数据库校准及其reload结果处更新，查询/reload失败不得清零或改用Redis/hint revision。
- [x] 4.3 更新watcher结构化日志为适用的`database_latest_policy_revision`、`local_applied_policy_revision`、`target_revision`、`hint_revision`、`source`和稳定reason，删除把`remote_policy_revision`、`remote_version`或`version_check`描述为权威事实的字段。
- [x] 4.4 补充metrics与日志测试，覆盖reason/source allowlist、lag非负、lag为`0`时actual applied不低于已读database latest，以及无revision数值或原始错误进入metrics label；运行对应permission metrics测试。

## 5. dashboard、alert与runbook

- [x] 5.1 更新Grafana dashboard源中的RBAC reload lag panel标题、说明和查询语义，明确database latest与local applied projection差值，并重新生成Compose provisioning dashboard副本。
- [x] 5.2 更新Prometheus RBAC lag与相关failure alert、annotation和测试fixture，使持续非零database projection lag覆盖既定SLO，并关联revision store unavailable与reload failure诊断信号。
- [x] 5.3 更新`docs/observability/user-service-runbook.md`，说明Pub/Sub丢失、Redis counter缺失/落后/重建、数据库revision超前和reload失败后的排障与恢复，不保留旧Redis/local version lag语义。
- [x] 5.4 运行dashboard生成命令和`make compose-dashboard-check`，再使用`git diff --exit-code`检查重新生成后的dashboard/provisioning产物无二次drift；运行Prometheus rules与metrics load fixture校验。

## 6. 规格一致性与交付验证

- [x] 6.1 对照`specs/rbac-access-control/spec.md`和`specs/runtime-observability/spec.md`检查实现与测试，确认未新增policy revision/outbox schema、未实现dispatcher、未修改Casbin engine apply gate、未改造user-role cache generation且未保留Redis lag兼容路径。
- [x] 6.2 运行`make user-service-architecture-lint`，确认revision source、watcher与metrics语义留在permission feature且application/domain没有Ent concrete依赖。
- [x] 6.3 检查HTTP API、OpenAPI和数据库schema无变化；若实现意外产生OpenAPI或Ent/Atlas生成物diff，先修正越界变更，不以生成兼容产物掩盖。
- [x] 6.4 在全部代码、测试、观测资产、规格和文档任务完成后，仅暂存本change的预期文件，并检查`git status`与staged diff不包含无关或敏感变更。
- [x] 6.5 在预期变更已暂存后运行`make lint`；命令未运行或失败时不得勾选本任务或将change标记完成。
- [x] 6.6 在预期变更已暂存且lint通过后运行`make verify`；命令未运行、失败或最终drift检查不通过时不得勾选本任务或将change标记完成。
