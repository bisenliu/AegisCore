## Context

AegisCore 当前由 `common/response` 定义统一 `Envelope` 和应用错误类型，`common/validation` 负责请求参数绑定与中文化校验错误，`common/middleware/recovery.go` 负责 panic 恢复，`user-services` 的 controller/service/repository 通过 `response.Fail`、`response.FromError` 和 `response.NotFoundError` 输出失败响应。现状中 `response.Code` 是字符串，标准错误类别较少，参数校验失败与普通请求错误都复用 `BAD_REQUEST`，服务侧业务文案也散落在具体调用点。

本变更属于 `api-response-contract` 的横切契约调整，会影响 `common` 共享包、用户服务调用点和测试断言。它不涉及数据库 schema、Ent 生成代码、Atlas migration、Redis/PostgreSQL 连接或服务启动依赖。

## Goals / Non-Goals

**Goals:**

- 将响应体 `code` 标准化为数字业务码，并保留 HTTP status 的标准语义。
- 扩展通用错误类别，覆盖请求错误、参数校验失败、未认证、无权限、冲突、未找到和内部错误。
- 提供可变参数错误构造 helper，让固定文案和格式化模板使用同一套 API。
- 区分参数校验失败和普通请求错误，保持 `common/validation` 只处理请求参数校验。
- 将 `user-services` 业务错误文案集中到 `internal/apperror`，服务侧继续复用 `common/response` 的错误构造。
- 更新测试覆盖响应码、消息、格式化行为和内部错误包装。

**Non-Goals:**

- 不新增认证、授权、用户写入、支付或其他业务能力。
- 不引入错误码注册中心、国际化资源系统或动态错误文案配置。
- 不改变 `Envelope` 的整体字段结构，除 `code` 字段类型和值外不调整 `success`、`message`、`data` 的语义。
- 不改变 controller/service/repository 分层职责。

## Decisions

1. `response.Code` 使用 `int`，而不是自定义 JSON 字符串编码。

   数字业务码是调用方需要的稳定分类键，直接用 `int` 可以让 JSON 输出为数字，避免额外 marshal 逻辑。备选方案是保留字符串码并新增数字字段，但会让响应信封长期携带两套分类键，增加调用方歧义。

2. 通用码最小集合放在 `common/response`。

   `common/response` 是所有服务共享的响应契约边界，通用码应与通用 helper 同处一包。服务侧只维护业务文案，不定义 `BadRequest`、`Conflict`、`NotFound` 等重复 helper。备选方案是在 `user-services` 中定义服务级错误码，但当前只有通用响应契约需要标准化，过早引入服务级码表会增加维护成本。

3. 可变参数格式化由一个内部函数统一处理。

   `BadRequestError(format string, args ...any)`、`ConflictError(format string, args ...any)` 等 helper 通过共享格式化函数生成 message。没有 `args` 时必须直接返回 `format`，不调用 `fmt.Sprintf`，避免固定文案中包含 `%` 时被误解析。备选方案是提供 `BadRequestErrorf`，但会让 API 分裂，不符合固定文案和格式化模板共用同一函数的目标。

4. 参数校验错误返回 `CodeValidationFailed`，绑定解析错误返回 `CodeBadRequest`。

   `go-playground/validator` 产生的 required、min/max、len、email、enum、gt/gte/lt/lte 等规则错误属于参数校验失败。JSON 类型不匹配、URI/query/form 解析类型错误、空请求体或格式错误属于请求错误。该边界能让调用方区分“字段规则不通过”和“请求无法被正确解析”。

5. `WrapInternal(err, publicMessage)` 替代只返回固定内部错误文案的单一路径。

   内部错误应保留 cause 供日志和排查使用，同时响应安全的对外文案。`InternalError(err)` 可以在实现时保留为兼容内部调用的薄封装，但主契约以 `WrapInternal` 为准。`response.FromError` 继续将普通 Go error 包装为内部错误，默认 public message 使用 `internal server error`。

6. `user-services/internal/apperror` 只放文案常量和模板。

   该包不封装响应 helper，不依赖 Gin，也不承担业务分支逻辑。repository 可使用 `response.NotFoundError(apperror.MsgUserNotFound)`，service 可使用 `response.BadRequestError(apperror.MsgMinMaxValue, ...)` 或 `response.WrapInternal(cause, ...)`。这保持业务文案集中维护，同时避免破坏分层。

## Risks / Trade-offs

- 响应体 `code` 类型变化会破坏已按字符串码解析的调用方。→ 在 proposal、spec 和测试中明确这是 breaking change，并以数字码断言固定契约。
- `common/validation` 错误分类不当可能把解析错误误报为校验失败。→ 实现时根据现有 `Error`/validator 错误类型区分，增加测试覆盖 validator 失败与类型解析失败。
- 可变参数 helper 如果无参数仍调用 `fmt.Sprintf`，固定文案中的 `%` 可能输出异常。→ 抽取格式化函数并增加包含 `%` 的固定文案测试。
- `WrapInternal` 如果直接暴露 `err.Error()` 会泄漏内部信息。→ 函数签名显式要求 `publicMessage`，`FromError` 默认仍使用 `internal server error`。
- 服务侧 `apperror` 可能演变成业务错误工厂。→ tasks 中限定仅新增常量和模板，不新增响应 helper。

## Migration Plan

1. 在 `common/response` 修改 `Code` 类型和值，增加通用错误 helper、HTTP 状态映射和 `WrapInternal`。
2. 调整 `common/validation` 的错误响应分类，使 validator 规则错误返回 `CodeValidationFailed`，解析类错误返回 `CodeBadRequest`。
3. 新增 `user-services/internal/apperror` 并迁移当前用户不存在等业务文案。
4. 更新 controller/service/repository 调用点和测试断言。
5. 分别在 `common/` 与 `user-services/` 执行 `go test ./...`。

Rollback 策略是回退本变更涉及的 `common/response` 契约、`common/validation` 分类和测试断言；由于不涉及数据库或持久化数据，无需数据迁移回滚。

## Open Questions

- 暂无。当前最小通用码集合可满足已有用户查询 API 和后续常见业务错误分类，服务级细分码可在具体业务能力出现时单独提出。
