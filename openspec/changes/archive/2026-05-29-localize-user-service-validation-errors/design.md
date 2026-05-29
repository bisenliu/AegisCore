## Context

`common/validation` 已经提供中文校验消息：默认 locale 为 `zh`，`validator/v10` 的中文翻译已注册，JSON 类型错误、空请求体和结构体字段校验也会被归一化为 `validation.Error`。用户查询 controller 当前调用 `ctl.validator.Bind(c, &req, validation.URIBinder)`，但在返回错误时只打印调试信息并调用 `response.BadRequest(c, "invalid user id")`，导致共享校验器的中文消息没有进入 HTTP 响应。

该问题属于 `user-profile-query` 能力的外部响应语义：参数错误仍然是 HTTP 400 和 `BAD_REQUEST`，但 `message` 应来自共享校验器的公开消息，而不是 controller 自行硬编码。

## Goals / Non-Goals

**Goals:**
- 让 `GET /api/v1/users/:id` 的无效 ID 响应使用共享校验器生成的中文公开错误消息。
- 保持 controller/service/repository 分层：controller 只负责参数绑定、错误响应和业务调用，不把校验翻译逻辑下沉到 service 或 repository。
- 保持统一响应信封、HTTP 状态码和错误码不变。
- 用单元测试覆盖非数字 ID 和非正数 ID 的响应消息来源。

**Non-Goals:**
- 不新增认证、授权、用户管理或其他用户服务 API。
- 不修改 Ent schema、Atlas migration、数据库结构或运行时配置。
- 不引入新的 i18n 框架，也不在 `user-services` 中重复实现校验翻译。
- 不改变 not found 和内部错误的现有响应语义。

## Decisions

- controller 复用 `validation.Validator` 的公开错误消息，而不是继续硬编码 `invalid user id`。原因是 `common/validation` 是跨服务共享校验能力，已经包含字段名解析、中文翻译和错误归一化；复用它可以避免不同 controller 产生不一致错误文本。备选方案是在 `user-services` 中写本地错误映射，但会重复 common 的责任并增加后续维护成本。
- 参数绑定失败仍通过 `response.BadRequest` 输出。原因是这一路径只改变 `message` 文本，不改变响应信封契约；`response.BadRequest` 已稳定输出 HTTP 400 和 `BAD_REQUEST`。备选方案是改用 `validation.BindOrAbort`，但当前 controller 需要保留显式 early return 结构，最小改动是保留 `Bind` 并将错误消息传给 response。
- 移除 `fmt.Println` 调试输出，必要日志由共享校验 helper 或后续结构化日志承担。原因是 controller 中直接打印会污染 stdout，且不携带 trace-id。备选方案是改为结构化日志，但本变更目标是响应本地化，不扩大日志行为。
- 测试以 controller 单元测试为主，验证响应消息不再是英文硬编码。原因是 `common/validation` 已有自身单元测试覆盖中文翻译，用户服务侧需要验证 controller 没有丢弃该错误。备选方案是增加端到端 HTTP server 测试，但需要更多运行时组装，不符合本次最小变更。

## Risks / Trade-offs

- 错误消息文本变化可能影响依赖英文 `invalid user id` 的客户端。缓解方式：保持 HTTP 400、`BAD_REQUEST` 和信封结构不变，并在规格中明确这是外部可观察文本变化。
- URI 绑定的非数字错误可能来自 `strconv.ParseInt` 包装错误，若未被 `common/validation` 归一化，可能仍暴露英文底层错误。缓解方式：实现时检查 `validation.URIBinder` 的绑定错误归一化路径，并在必要时扩展 `common/validation` 的 bind 错误中文化测试。
- 非正数 ID 与非数字 ID 的中文消息可能不同。缓解方式：测试分别覆盖两个场景，只断言符合共享校验器输出的中文公开消息，而不是强行统一为同一句固定文案。
