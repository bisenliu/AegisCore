## Context

`common/runtime/scheduler/lifecycle.go` 当前包含 `New`、`Start`、`Add`、`Remove`、`Stop`、drain helper 和完整使用说明。相邻的 `localcache`、`workerpool` 已将完整契约放在 `doc.go`，把可运行用法放在 `example_test.go`，并按执行、生命周期和统计等职责拆分生产文件。

`Job` 按值传入 `Add`，但其 `Lock` 和 `Lock.Renew` 是指针。当前 `validateJob` 会在这些嵌套对象上填充默认值，因而修改调用方对象；cron closure 也长期引用相同嵌套指针。调用方在 `Add` 返回后复用或修改配置时，会改变已注册任务的锁 TTL、等待或续租策略。

duration 默认化目前同时接受零值和负数。公开契约只将零值定义为默认值，负数没有合理运行时语义，应在构造或注册边界拒绝。

## Goals / Non-Goals

**Goals:**

- 保持 scheduler package、导出类型、函数、方法、错误和正常调用方式不变。
- 按 package docs、构造、注册、生命周期和 executable examples 整理同 package 职责。
- 让每个成功注册的 job 持有独立的 `LockPolicy` 与 `RenewPolicy` 快照，不修改或继续引用调用方嵌套配置。
- 仅让 duration 零值触发默认值，统一拒绝负数 lock、renew 和 retry duration。
- 保持 cron parser、overlap/global gate、Redis owner lock、retry、renew、metrics、panic recovery 和 Stop drain 语义不变。

**Non-Goals:**

- 不新增 Stats、手动触发、持久化队列、exactly-once、fencing、Fx provider 或公开 middleware。
- 不改变 metrics event、status、reason、duration 或日志字段契约。
- 不更换 `robfig/cron`、`go-redis` 或 miniredis 依赖。
- 不修改 user-service feature、HTTP API、数据库、OpenAPI、部署或观测资产。

## Decisions

### Decision: 在归一化入口创建深层策略快照

新增包内 `normalizeJob(Job) (Job, error)`，先复制 `Job`，再分别复制可选 `LockPolicy` 和 `RenewPolicy`，最后只在副本上裁剪字符串、填充默认值并校验。`Add` 构造 pipeline 和 cron closure 时只捕获归一化后的副本。

备选方案是在 `Add` 完成后禁止调用方修改原对象。Go API 无法可靠执行该约束，也不能消除默认化修改调用方对象的副作用，因此不采用。另一方案是把策略改为非指针并增加 enable 字段，会扩大公开 API 并重新引入矛盾状态，因此不采用。

### Decision: 零值默认与负值错误严格分离

`Config.DefaultLockTTL`、`LockPolicy.TTL`、`RenewPolicy.Interval`、`RenewPolicy.Timeout`、`RetryPolicy.InitialInterval` 和 `RetryPolicy.MaxInterval` 的负数值返回包装 `ErrInvalidLock` 的错误。仅精确零值使用既有默认值；其他范围关系校验保持不变。

备选方案是继续用 `<= 0` 默认化。该方案会隐藏配置拼写、单位或计算错误，且与 `Timeout`、`WaitTimeout`、`MaxAttempts` 的负值拒绝方式不一致，因此不采用。

### Decision: 完整契约进入 package 文档

`doc.go` 承载配置层级、cron 格式、执行顺序、context、并发、Redis lock/retry/renew、关闭、观测和能力边界。`New` 只保留简洁摘要和文档指引。`example_test.go` 使用公开 API 展示基本生命周期、分布式锁和取消，不依赖未导出测试入口。

构造、注册和生命周期继续位于同一 package，但分别放入 `scheduler.go`、`registration.go` 和 `lifecycle.go`。不创建子包，不改变导入路径或包内所有权。

## Risks / Trade-offs

- [Risk] 文件移动遗漏注释、import 或 helper。-> Mitigation：保持同 package，运行 `gofmt`、`go vet`、普通测试和 race 测试。
- [Risk] 防御性复制遗漏新增嵌套可变字段。-> Mitigation：复制逻辑集中在 `normalizeJob`，并通过调用方对象不变与注册后修改隔离测试覆盖当前全部嵌套策略。
- [Risk] 严格拒绝负数暴露此前未发现的无效配置。-> Mitigation：当前仓库无生产注册者，零值默认和正数配置不变，错误继续通过既有 `ErrInvalidLock` 匹配。
- [Risk] 当前未归档 scheduler change 也修改同一 capability。-> Mitigation：本 change 使用独立 ADDED requirement，不修改或代为归档现有 change。

## Migration Plan

1. 先增加配置快照和负值校验测试，再修改归一化实现。
2. 将 package 文档与 examples 从构造函数注释中拆出，再按职责移动同 package 声明。
3. 运行 scheduler 普通测试、race、vet、OpenSpec、架构、lint 与 verify 门禁。
4. 本变更没有调用方或部署迁移。回滚时可恢复原文件布局和校验实现，不涉及持久化数据或外部资源回滚。
