## Context

`common/runtime/logger` 暴露 `SetDefault` 作为进程级兜底 logger 能力，`FromContext` 在 context 中没有 logger 时会读取该默认值。当前测试中既有专门验证默认 logger 的用例，也有为了捕获日志而临时替换默认 logger 的用例。后者会扩大进程级状态影响范围，尤其在跨包测试、并行测试或日志捕获断言中容易产生串扰。

受影响路径限定在 `common/runtime/logger/context.go`、`common/runtime/logger/logger_test.go` 和 `common/http/binding/validation_test.go`。本次不涉及 HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 将非必要的日志捕获测试改为使用 `logger.ToContext` 或显式局部 logger。
- 仅在验证默认 logger 兜底行为时调用 `SetDefault`。
- 为必须调用 `SetDefault` 的测试集中提供保存和恢复 helper，并避免这些测试并行运行。
- 保持 `FromContext`、`WithContext`、`SQL`、`SetDefault` 的生产行为和日志字段契约不变。

**Non-Goals:**

- 不删除或废弃生产 API `SetDefault`。
- 不改变 `trace_id`、`span_id`、logger name、日志 level 或 message。
- 不引入覆盖所有测试的全局锁，也不为了测试新增生产分支、接口或 adapter。
- 不修改 OpenAPI、数据库 schema、部署资产或 user-service 业务代码。

## Decisions

### Decision: 日志捕获优先使用 context logger

对不需要验证进程级兜底 logger 的测试，优先构造局部 zap logger 并通过 `logger.ToContext` 注入 request context 或调用 context。这样可以直接覆盖生产代码从 context 取 logger 的路径，同时不修改包级默认值。

备选方案是继续使用 `SetDefault` 并在每个测试中 cleanup。该方式仍会在测试执行窗口内污染进程级状态，不能消除并行测试串扰。

### Decision: `SetDefault` 测试使用保存和恢复 helper

专门验证默认 logger 行为的测试仍需要替换默认 logger。为这些测试增加测试 helper，先保存当前默认值，替换为测试 logger，并在 cleanup 中恢复保存值。helper 只放在 `common/runtime/logger` 的测试文件内，不作为生产 API 暴露。

备选方案是在生产代码新增 `GetDefault` 或测试专用入口。该方案会扩大生产 API 面，不符合本次只收敛测试影响的目标。

### Decision: 不让默认 logger 相关测试并行

凡是调用 `SetDefault` 的测试不使用 `t.Parallel()`，并在 helper 命名和测试组织上表达其进程级状态特性。其他只使用局部 logger 或 context logger 的测试可以继续保持原有运行方式。

备选方案是新增一个全局测试锁包裹所有 logger 相关测试。该方案会掩盖测试设计问题，并降低测试并发能力，因此只保留最小 helper 内的保存/恢复行为。

## Risks / Trade-offs

- [Risk] `common/http/binding` 测试如果未正确把 logger 注入 Gin request context，可能无法捕获预期日志。-> Mitigation：在测试中显式替换 request context，并继续断言日志内容。
- [Risk] 保存和恢复默认 logger 的 helper 依赖同包测试访问未导出默认值读取函数。-> Mitigation：helper 留在 `package logger` 测试内，不新增生产导出 API。
- [Risk] 只运行单包测试可能遗漏跨包影响。-> Mitigation：按任务运行 `go test -race ./runtime/logger` 与 `go test -race ./http/binding`，并保留 OpenSpec 与架构 lint 验证。

## Migration Plan

1. 新增 OpenSpec delta，记录 logger 默认值测试隔离约束。
2. 调整 `common/runtime/logger` 测试 helper 和必须使用 `SetDefault` 的用例。
3. 调整 `common/http/binding` 日志捕获测试，改为 request context logger 注入。
4. 运行 `openspec validate isolate-logger-default-state-tests`、`make user-service-architecture-lint`、`go test -race ./runtime/logger` 和 `go test -race ./http/binding`。

回滚方式是撤销本次测试和 OpenSpec change 文件，不需要数据库、部署或运行时迁移。

## Open Questions

- 无。
