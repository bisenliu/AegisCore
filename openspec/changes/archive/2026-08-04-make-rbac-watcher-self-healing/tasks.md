## 1. 配置与状态契约

- [x] 1.1 在 `user-service/internal/config/` 新增 `rbac.policy_watcher` 的 `check_interval`、`subscribe_timeout`、`max_staleness` 和 `retry_backoff` 正式配置、默认值、settings 投影及校验，更新两套 Nacos 配置基线和严格解码/默认值/非法值测试，不增加旧配置别名或回退分支。
- [x] 1.2 在 permission application 中以结构化 `PolicyWatcherStatusSnapshot` 和 `Status()` port 替换 `Running()`/`LastError()`，定义 subscription state 与低基数错误类别，并同步 Fx runtime/provider 和测试 stub，删除旧状态接口与 adapter。

## 2. Watcher 自恢复实现

- [x] 2.1 重构 permission Redis watcher 的生命周期所有权：删除 `PubSub.Channel()` 消费路径，建立可取消的 subscription supervisor、订阅确认 timeout、单 owner PubSub 关闭和带抖动的有界指数退避，确保确认或 Receive 失败后持续重建订阅。
- [x] 2.2 建立启动立即执行且独立于订阅状态的 PostgreSQL revision 周期校准，并通过 context-aware 内部 payload 队列串行执行消息处理与 revision reload，保证订阅退避期间补偿继续运行且 projection 不并发倒退。
- [x] 2.3 实现 subscription/reconcile 独立状态转换：成功确认或权威校准只清除对应当前错误，严格按数据库查询及最终 projection ready/applied 条件推进成功时间，正常取消不记录故障。
- [x] 2.4 完成 `Stop` 和 Fx 启动回滚的同步退出，确保确认等待、Receive、退避 timer、payload 交付和校准 ticker 均响应根 context，当前 PubSub 恰好关闭一次且共享 Redis client 保持开放。

## 3. 健康与指标

- [x] 3.1 重写 watcher health checker：启动后首次权威校准前保持 unavailable，运行且校准年龄不超过 `max_staleness` 时 available，stopped/never synchronized/stale 时返回稳定 503 诊断，并保持 `/livez` 语义不变。
- [x] 3.2 在 permission/user-service 观测边界实现 watcher 专用低基数 collector，暴露 running、subscription connected、最后订阅成功时间、最后权威校准成功时间、reconcile staleness 和重连尝试，删除 watcher 对通用 component running/last error 指标的注册与查询，不双写旧 series。
- [x] 3.3 更新 health、metrics 和 Fx 装配测试，覆盖 reconnecting 但校准新鲜、首次未校准、staleness 临界值、恢复后状态清除、metrics 禁用和固定 label 枚举。

## 4. 故障注入与并发验证

- [x] 4.1 扩展 watcher 可控 subscriber 测试，使用通道、barrier、明确 deadline 和 eventually 条件覆盖初始订阅失败后恢复、持续失败有界退避、运行期 Receive 终止后重订阅及恢复后消息消费，不以固定 `time.Sleep` 证明状态变化。
- [x] 4.2 覆盖订阅重建期间数据库 revision 补偿继续收敛、查询恢复和 reload 恢复分别清错、成功时间只在完整校准后推进，以及重复/乱序 hint 不使 projection 倒退。
- [x] 4.3 覆盖 Stop 在订阅确认、Receive、退避和 payload 阻塞各阶段取消，断言无停止后重订阅、每个 PubSub 恰好关闭一次，并运行 `go test -race ./internal/features/permission/infrastructure/redis ./internal/features/permission ./internal/providers/observability` 验证数据竞争与 goroutine 生命周期。

## 5. 部署观测与文档

- [x] 5.1 更新 `deployments/observability/prometheus/user-service-alerts.yaml`，以新 watcher running、staleness 和 reconnect 指标分别表达 stopped/stale critical 与持续重连 warning，删除旧 watcher component series 查询并更新规则测试。
- [x] 5.2 更新通用 Grafana dashboard 的 watcher 当前状态、最后成功时间和 staleness 面板，运行 `make compose-dashboard-generate` 生成 Compose 副本，再运行 `make compose-dashboard-check` 验证无 drift。
- [x] 5.3 更新 watcher runbook、`docs/TESTING.md` 及相关架构/开发说明，记录故障状态、恢复条件、staleness 健康定义、发布顺序和回滚时先移除新 Nacos 配置键的要求。

## 6. 变更验证与交付门禁

- [x] 6.1 运行 permission Redis、permission feature、config、observability 和 router 相关包测试，并运行 `make user-service-architecture-lint`、`openspec validate make-rbac-watcher-self-healing` 与 `openspec validate --specs`；任一失败均不得继续交付。
- [x] 6.2 检查 `git diff`，确认没有 Ent/OpenAPI/数据库 migration 或无关生成物变化，并将本 change 的代码、配置、测试、观测资产、文档和 OpenSpec artifacts 全部加入暂存区。
- [x] 6.3 在预期变更已暂存后依次运行 `make lint` 和 `make verify`，确认最终生成物 drift 与 `git diff --exit-code` 门禁通过；未通过或未运行时不得标记本 change 完成。
