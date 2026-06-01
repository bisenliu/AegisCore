## Context

AegisCore 使用 Go 1.26 workspace，`common` 模块提供共享 validation、response 和 logger 能力。当前 `common/validation/validation.go` 已使用 `any` 定义主要绑定接口，但仓库内仍可能存在手写 Go 代码使用 `interface{}` 表达空接口的情况。

`BindOrAbort` 是 Gin controller 处理请求绑定和校验失败的集中出口。当前失败日志通过 `common/logger` 以 warning 级别输出，并包含 `error` 与 `path` 字段；当错误是 validator tag 校验失败时，响应会包含字段级 `errors` 明细，但日志没有同步记录这些明细。

## Goals / Non-Goals

**Goals:**

- 将手写 Go 代码中表示空接口的 `interface{}` 迁移为 `any`，保持 Go 1.26 代码风格一致。
- 将 `BindOrAbort` 的无效请求日志从 warning 调整为 error。
- 当归一化校验错误包含字段级明细时，在结构化日志中输出 `errors` 字段。
- 保持现有 request validation 绑定、校验、响应 envelope 和错误码行为不变。

**Non-Goals:**

- 不修改 `common/response.Envelope` 响应结构、HTTP 状态码或业务错误码。
- 不引入新的 validation 规则、翻译语言或 DTO tag 解析策略。
- 不修改 Ent schema、数据库 migration、Redis/PostgreSQL 配置或 Fx 装配。
- 不手动编辑 `user-services/ent/` 生成代码；如生成代码含 `interface{}`，本次变更不以手写方式修改。

## Decisions

1. 使用 `any` 替换空接口用途，而不是保留 `interface{}`。

   Go 1.26 已支持 `any`，且项目 toolchain baseline 明确使用 Go 1.26。`any` 与 `interface{}` 在类型系统上等价，不改变运行时行为；替换只影响代码可读性和一致性。

   Alternative considered: 只修改 validation 文件。该方案无法满足项目范围内统一空接口写法的目标。

2. 仅迁移手写代码中的空接口，不手写修改生成代码。

   `user-services/ent/` 由 Ent 生成，仓库规则禁止手写生成代码。若生成代码仍包含 `interface{}`，应通过生成器版本或配置变化解决，而不是本变更直接编辑。

   Alternative considered: 对所有 `.go` 文件做全量文本替换。该方案可能触碰生成代码，违反仓库规则。

3. 在 `BindOrAbort` 中构造日志字段列表，并按错误明细条件追加 `zap.Any("errors", details)`。

   `validationDetails(err)` 已返回字段级 `[]FieldError`，可直接作为结构化日志字段输出。仅当切片非空时追加 `errors`，避免无字段级明细的 JSON 类型错误、空 body 或其他 bad request 日志出现空字段。

   Alternative considered: 始终输出 `errors` 字段。该方案会让无字段级明细的错误携带空数组或 nil，降低日志信噪比。

4. 使用 `logger.Error` 输出无效请求日志。

   项目日志规则要求业务日志优先使用 `common/logger` context API。维持该 API 可以继续自动携带 `trace-id`，只调整日志等级和字段。

   Alternative considered: 直接使用底层 `zap.Logger`。该方案会绕过当前 context logger 约定，不利于 trace-id 一致性。

## Risks / Trade-offs

- [Risk] 将所有手写 `interface{}` 替换为 `any` 可能遗漏特殊格式或注释中的写法。→ Mitigation: 使用内容搜索定位 `interface{}`，人工区分类型用法与非代码内容，并运行 `gofmt` 与测试。
- [Risk] error 级别记录请求参数错误会增加 error 日志量。→ Mitigation: 该变化来自明确需求；只在集中校验失败出口调整，避免扩大到其他请求日志。
- [Risk] `errors` 字段包含用户输入关联的字段名和错误消息。→ Mitigation: 复用已用于 HTTP 响应的 `FieldError` 明细，不额外记录原始请求值。
- [Risk] 测试环境 Go 1.26 toolchain 不可用会阻塞验证。→ Mitigation: 记录无法运行的命令和原因；代码改动保持 Go 1.18+ 兼容语法。
