## Context

当前共享请求校验能力位于 `common/validation`，负责 Gin controller 的 URI、query、JSON、form 绑定和 validator tag 校验。校验失败时会归一化为 `validation.Error`，`BindOrAbort` 再调用 `common/response.ValidationFailed` 输出统一失败信封。

现有 `validation.FieldError` 只有 `field` 与 `message`，且响应层的 `Envelope` 没有 `errors` 字段，因此外部调用方只能看到顶层 `message: 请求参数验证失败`。本变更需要在不改变 controller/service/repository 分层和现有错误码的前提下，为结构体校验失败输出字段级明细。

## Goals / Non-Goals

**Goals:**

- 对 validator tag 校验失败返回 HTTP 400、业务码 `10001`、顶层消息 `请求参数验证失败`，并附加结构化 `errors` 数组。
- 每个字段错误包含 `field`、`label`、`rule`、`message`，支持前端定位字段并展示中文错误。
- 字段名使用请求字段名，显示名优先使用 DTO `label` tag，规则使用 validator tag，例如 `required`、`email`、`gt`、`enum`。
- 保持 `common` 作为共享能力边界，避免在 `user-services` controller 中重复拼装校验错误响应。

**Non-Goals:**

- 不改变请求体为空、JSON 类型不匹配、URI/query/form 类型解析失败等 bad request 错误的业务码和响应结构。
- 不新增认证、授权、用户业务规则或数据库 schema。
- 不要求所有业务自定义 `Validate() error` 都必须映射为字段级 `errors`；无法定位字段的自定义错误继续使用现有消息。
- 不调整 Swagger 文档能力，除非实现阶段发现已有文档测试必须同步最小示例。

## Decisions

- 在 `common/validation.FieldError` 中扩展字段级错误模型，而不是在 `user-services` DTO 或 controller 中定义专用结构。
  这样所有使用共享 validator 的 API 都能获得一致响应，符合共享基础能力优先放在 `common/` 的边界。
  备选方案是在 controller 中手工转换错误，但会造成服务侧重复实现并破坏统一校验能力。

- `field` 输出请求字段名，`label` 输出显示名。
  `field` 应从 DTO 的 `json`、`form`、`uri`、`query` tag 中解析，便于调用方映射表单字段；`label` 优先来自 `label` tag，缺失时回退为请求字段名或结构体字段名。
  备选方案是复用 validator 的 `Field()` 同时承担字段名和显示名，但当前 `RegisterTagNameFunc` 已让翻译消息偏向显示名，无法满足调用方需要稳定字段 key 的要求。

- `rule` 使用 validator 的 `Tag()`。
  这能稳定表达触发规则，例如 `email`、`required`、`gt`，无需把参数值拼入规则名。对于别名或复合规则，先保持 validator 返回的 tag，避免引入自定义规则命名层。

- 响应层新增可选 `Errors` 字段或等价 helper，`ValidationFailed` 能接收字段错误明细并输出 `errors`。
  顶层 `data` 保持空值并通过当前 JSON 规则省略或输出 null；若实现选择保留 `omitempty`，测试应以现有响应契约为准，只要求校验失败包含 `errors`。
  备选方案是把字段错误放入 `data`，但这会混淆成功数据与失败明细，不符合用户指定响应示例。

## Risks / Trade-offs

- [Risk] 扩展 `Envelope` 可能影响所有失败响应的 JSON 序列化。
  Mitigation: 将 `errors` 设计为可选字段，仅在参数校验失败且存在字段明细时输出。

- [Risk] `RegisterTagNameFunc` 当前让 validator 翻译消息使用标签名，新增 `field` 与 `label` 可能需要额外反射解析，存在嵌套字段处理差异。
  Mitigation: 为常见 DTO 字段、`label` 缺失、`json:"-"`、嵌套字段添加单元测试，先覆盖当前 API 所需行为。

- [Risk] 多字段校验失败时 `errors` 顺序依赖 validator 返回顺序。
  Mitigation: 不把顺序作为业务契约，仅保证每个失败字段都有一条结构化明细。

- [Risk] 自定义 `Validate() error` 无法自动提取字段、规则和 label。
  Mitigation: 本次只规范 validator tag 校验失败；未来如需要业务字段错误，可扩展专用错误类型。
