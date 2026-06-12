# Standardize logging and comments

## What

全面审计 `common/` 和 `user-service/` 非生成 Go 代码中的日志输出与代码注释，形成统一规范并完成代码落地：

- 补全缺失的关键日志，尤其是启动停止、外部资源访问、认证会话状态变更、后台任务、降级 fallback 和异常路径。
- 更正日志级别，使 `Debug`、`Info`、`Warn`、`Error` 与场景严重性匹配。
- 将代码中的英文注释统一改为中文，函数和方法注释也必须使用中文。
- 保持所有日志消息内容为英文，避免中英文混杂影响检索、告警和跨环境运维。
- 将“函数注释必须全部使用中文；log 日志内容必须全部使用英文”补充到长期主规则中，并在本次实现中统一执行。

本变更不修改业务流程、HTTP API、数据库 schema、迁移、配置 key 或生成代码语义。

## Why

当前仓库已经具备统一 Zap logger、trace-id middleware、HTTP request logging、worker pool 日志和 application 层关键路径日志，但日志覆盖和级别语义仍需要一次系统整理：

- 部分错误路径已经返回 error，但缺少足够上下文日志，排障时需要沿调用链推断。
- 部分预期性业务拒绝、客户端输入问题、运行时异常和后台任务失败应有清晰等级边界。
- 注释语言混用会降低维护一致性，尤其是面向中文协作团队时，函数注释和复杂逻辑说明应统一为中文。
- 日志内容需要保持英文，便于生产环境检索、聚合、告警规则、仪表盘和跨团队交接。

把这些规则写入主规格并统一落地，可以减少后续功能开发中的日志漂移和注释风格反复。

## Scope

包括：

- 更新 `AGENTS.md` 和 `docs/ARCHITECTURE.md`，补充代码注释与日志内容语言规则。
- 审计 `common/` 和 `user-service/` 下非生成 Go 源码中的日志调用。
- 为关键缺失场景补充结构化日志字段，优先使用 `common/runtime/logger` 的 context helper 保留 trace-id。
- 调整日志等级：
  - `Debug`：生命周期细节、低价值调试信息、可重复推导的内部状态。
  - `Info`：服务启动停止、外部资源连接关闭、成功完成的重要业务动作。
  - `Warn`：客户端或业务可预期拒绝、认证失败、缓存降级、幂等冲突、非致命提交拒绝。
  - `Error`：系统异常、外部依赖失败、数据访问失败、后台任务失败、panic recover 和需要运维关注的问题。
- 将非生成 Go 源码中的英文注释改为中文，包括 package 注释、类型注释、函数/方法注释、复杂逻辑注释和测试 helper 注释。
- 确认新增或修改的日志消息全部为英文，字段名继续使用稳定英文 snake_case。
- 更新相关测试中对日志消息、注释样例或 helper 输出的断言。

不包括：

- 不手写 `user-service/ent/` 下 Ent 生成代码，也不为翻译生成代码注释而直接修改生成产物。
- 不修改第三方、vendor、缓存目录或工具生成文件中的英文注释。
- 不改变 HTTP response message、error string、配置字段、数据库字段、Redis key、Swagger schema 名称或 Go identifier。
- 不引入新的日志库、metrics、tracing exporter 或告警系统。
- 不新增 OpenSpec/OPSX 工件。

## Acceptance Criteria

- 主规则明确要求：函数和方法注释必须使用中文；代码注释统一使用中文；日志消息内容必须使用英文。
- 非生成 Go 源码中不再存在需要人工维护的英文注释。
- 所有新增和保留的日志消息均为英文，日志字段名保持英文 snake_case。
- 日志级别与场景匹配：预期业务拒绝不使用 `Error`，系统异常不降级为 `Info`。
- 关键错误路径包含足够结构化字段，例如 `user_id`、`session_id`、`username`、`resource name`、`route`、`status`、`task`、`pool`、`error` 等。
- 优先通过 context logger 输出业务日志，避免丢失 trace-id。
- 生成代码、第三方代码和历史 change 文档不被误改。
- `gofmt` 后相关模块测试通过，至少运行 `make test-common` 和 `make test-user-service`。
