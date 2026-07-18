## Context

Fx `v1.24.0` 的 `OnStop` hooks 会按注册逆序串行执行，并共享 `App.Stop(ctx)` 传入的总 deadline。当前 runtime 配置只校验 `runtime.lifecycle.stop_timeout` 不小于 `server.http.shutdown_timeout` 与 `server.grpc.shutdown_timeout`，但 user-service 停止路径还包含 pprof、RBAC watcher、RBAC cache、auth purge worker、auth local cache、Ent、Redis、PostgreSQL、tracing flush/shutdown 和 logger sync。

本 change 跨越 `common/runtime/config` 的共享配置校验、`common/runtime/workerpool` 的停止语义约束、user-service 的 lifecycle 组装和 auth purge worker 关闭路径。它不改变对外 API、数据库 schema、OpenAPI 生成物、部署清单或安全授权边界。

## Goals / Non-Goals

**Goals:**

- 在共享 runtime 配置中建立可验证的最小停止预算，避免单个 hook 合法但串行总预算不足。
- 将 HTTP drain、auth purge worker drain、tracing flush allowance 和 safety margin 组合为明确的 `runtime.lifecycle.stop_timeout` 下限。
- 保持 `common` 业务中立，不把 auth、RBAC 或 user-service 专属生命周期语义下沉到共享 primitive。
- 用完整 App lifecycle recorder 测试验证关键关闭顺序，覆盖 HTTP/pprof、RBAC watcher 与 feature worker、Ent、Redis/PostgreSQL、tracing、logger。
- 明确停止错误不会阻止其余 Fx stop hooks 继续执行，测试和实现聚焦共享 context deadline 消耗风险。

**Non-Goals:**

- 不改变 Fx lifecycle 的执行模型或引入自定义 stop orchestrator。
- 不修改 HTTP API、OpenAPI 文档、数据库 schema、Atlas migration、RBAC policy 或部署资源。
- 不为测试新增无运行时职责的生产 API、全局可变 hook 或 feature 跨层 adapter。
- 不把 auth purge 的业务策略、Redis key schema 或 session 语义放入 `common/runtime/workerpool`。

## Decisions

- 在 `common/runtime/config` 增加组合预算校验。校验公式使用共享配置已经拥有的 HTTP shutdown timeout，并加上业务中立命名的 worker drain allowance、tracing flush allowance 和 safety margin 常量或字段默认值；这样失败会在配置加载阶段暴露。备选方案是在 user-service 私有配置中校验，但会遗漏未来服务复用 runtime lifecycle 时的相同风险。
- auth purge worker 的 30 秒停止上限继续归 auth owning package 或调用侧拥有，只把可组合的 drain allowance 作为预算输入表达，不把 session purge 语义放入 `workerpool`。备选方案是在 `workerpool.Stop` 内部硬编码默认超时，但这会让共享 primitive 承载调用方业务预算。
- lifecycle recorder 测试放在 user-service bootstrap 或 providers 层，使用真实 Fx App hook 顺序和轻量 recorder 资源验证逆序关闭。备选方案是只做单包单元测试，但无法覆盖跨资源顺序和共享 deadline。
- 对 pprof、tracing 和 logger 保持现有 `OnStop` hook 职责，只验证它们在关键资源之后仍会被调用并使用剩余 context。备选方案是给每个 hook 创建独立 context，但这会突破 `App.Stop(ctx)` 总预算并可能导致停机耗时无界增长。

## Risks / Trade-offs

- 预算下限过保守导致现有环境配置启动失败 → 使用与当前默认值兼容的默认 stop timeout 或同步调整默认值，并在错误信息中输出最低所需预算。
- worker drain allowance 与 auth purge 实际停止上限漂移 → 将常量命名和测试放在 owning package 附近，并在配置校验测试中覆盖公式。
- lifecycle recorder 测试过度依赖内部注册细节 → 只断言关键类别顺序，不断言不相关 hook 或完整 provider 列表。
- 某些 shutdown hook 返回 error 造成测试误判 → 测试明确覆盖普通 error 后 Fx 继续执行后续 stop hooks 的行为，重点断言前序 hook 会消耗后续 hook 的共享 deadline，而不是给后续 hook 新建完整预算。
- 增加校验可能改变启动失败时机 → 这是预期行为，回滚方式是移除组合预算校验并恢复原有默认值；不涉及数据迁移或外部契约回滚。

## Migration Plan

- 实现阶段先更新 OpenSpec delta 与测试，再更新配置默认值和校验逻辑。
- 如果默认 `runtime.lifecycle.stop_timeout` 小于新公式，调整默认值，使未覆盖该配置的开发和测试环境继续可启动。
- 部署时只需要发布新的 user-service/runtime 代码；不需要数据库 migration、OpenAPI 生成、部署清单或观测资产变更。
- 回滚时恢复旧校验和默认值即可，不需要数据修复。

## Open Questions

- 无。
