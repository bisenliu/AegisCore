## Context

`common/contract/errors` 当前核心模型为 `Error{Code, Message, HTTPStatus, Cause}`，构造函数在创建应用错误时直接写入 HTTP status。`common/http/response` 读取该字段后写出 HTTP 响应，导致共享应用错误契约和 HTTP 渲染责任耦合；`common/http/binding`、`common/http/middleware`、`common/validation` 也依赖旧的 code/status 驱动构造方式。

本变更跨越 `common/contract/`、`common/http/` 和 `common/validation/`，属于共享契约不兼容变更。它不改变数据库 schema、Ent migration、部署资产或 OpenAPI 生成物；如 HTTP 响应 envelope 字段和注解未变化，不需要重新生成 OpenAPI。

## Goals / Non-Goals

**Goals:**

- 将 `common/contract/errors` 重构为语义驱动的应用错误模型，稳定暴露 `Kind`、`Reason`、`Code`、`Message` 和可选 `Cause`。
- 移除 `HTTPStatus` 字段、接收 HTTP status 的旧 factory API，以及任何把旧模型转换到新模型的兼容适配层。
- 由 `common/http/response` 根据 `Kind` 统一推导 HTTP status，并保持统一 response envelope 输出。
- 保持 `errors.As` 能识别 wrapped application error，保持 `errors.Is` 可按稳定语义匹配应用错误类别或原因。
- 调整 binding、middleware 和 validation 共享路径，使其创建或传播语义错误，而不是构造 status 驱动错误。
- 用测试覆盖 nil error、未知 error、wrapped application error、validation error 和内部错误脱敏。

**Non-Goals:**

- 不迁移 user/auth/role/permission 的领域 sentinel error。
- 不删除各 feature 的 `toXHTTPError` mapper；这些 mapper 只需要在实现阶段适配新的共享错误构造方式。
- 不调整登录强制改密响应结构和 `CodePasswordChangeRequired` 的稳定 code 值。
- 不保留旧 `HTTPStatus` 字段、旧 status 参数 factory、旧错误响应兼容路径或旧状态码直连模型。
- 不新增数据库 migration、外部依赖、部署清单或观测 dashboard。

## Decisions

### Decision: `Kind` 承载错误类别，`Reason` 承载具体原因

应用错误模型应将通用类别和具体原因拆开：`Kind` 用于表达请求错误、校验失败、未认证、无权限、冲突、未找到、服务不可用、内部错误等低基数类别；`Reason` 用于表达 token invalid、token expired、password change required、empty request body、trailing JSON body 等稳定业务或边界原因。`Code` 继续作为 response envelope 的稳定应用响应码，但不再暗含 HTTP status。

备选方案是只保留 `Code` 并在 HTTP 层按 code 映射 status。该方案仍把错误分类绑定到历史 code 编号，无法清晰表达多个 reason 共享一个 HTTP 类别，也会让未来非 HTTP 消费方继续从 code 推断传输语义，因此不采用。

### Decision: HTTP status 只在 `common/http/response` 推导

`common/contract/errors` 不保存、不返回、不暴露 HTTP status。`common/http/response` 提供包内状态码映射函数，根据 `Kind` 写出 `400`、`401`、`403`、`404`、`409`、`500`、`503` 等状态码；未知、nil 或内部错误默认按内部错误脱敏并渲染为 `500`。

备选方案是在错误构造函数中保留一个可选 status 字段作为过渡。该方案会延长旧模型生命周期，并违背“最终不保留旧 `HTTPStatus` 字段、旧状态码直连模型或兼容适配层”的约束，因此不采用。

### Decision: 不提供旧 API 兼容层

实现阶段应删除接收 status 的 `NewError`、`Wrap` 或等价旧入口，并将调用方迁移到语义构造入口。可以保留不接收 status、语义明确的便捷 helper，但这些 helper 必须由 `Kind` 和 `Reason` 驱动，不能只是包装旧 API。

备选方案是保留旧函数并标记 deprecated。该方案会让新旧模型并存，增加调用方继续写入 HTTP status 的机会，因此不采用。

### Decision: wrapped error 支持基于 Go 标准错误链

应用错误继续通过 `Unwrap` 暴露 `Cause`，使 `errors.As` 可以从包装链中识别 `*errors.Error`。为 `errors.Is` 增加语义匹配能力，目标应用错误只声明 `Kind` 时可匹配同类别错误，声明 `Reason` 时必须匹配同 reason；内部 `Cause` 仍通过标准 `errors.Is/As` 链路工作。

备选方案是仅依赖 sentinel error。该方案会把共享错误模型和各 feature 的领域 sentinel 绑定起来，超出本次“不迁移领域 sentinel error”的范围，因此不采用。

### Decision: validation 保留字段明细，但错误类别改为语义字段

`common/validation.Error` 仍负责携带公开 message 与字段明细，供 `common/http/binding` 输出结构化校验错误。其错误分类不再保存旧 `Code` 作为唯一判断依据，而应携带或包裹语义应用错误信息，使 binding/response 能区分字段校验失败和通用 bad request，同时仍输出现有 `errors` 字段。

备选方案是让 `contract/errors.FromError` 直接导入并识别 `common/validation` 类型。该方案会让底层契约包依赖校验包，扩大 `common/contract` 依赖方向，因此不采用。

## Risks / Trade-offs

- [Risk] 旧 factory 调用点较多，删除 status 参数会造成编译失败范围扩大。Mitigation：先改 `common/contract/errors` 和 `common/http/response`，再按编译错误逐步迁移 `common/http/binding`、`common/http/middleware`、`common/validation` 及 feature mapper 调用点。
- [Risk] `Kind` 与 `Reason` 设计过细会产生高基数或业务泄露。Mitigation：`Kind` 只保留低基数类别；`Reason` 使用稳定、可公开的业务中立或业务边界原因，不记录动态资源 ID、内部依赖名或敏感细节。
- [Risk] 内部错误可能在未知 error 归一化时泄露原始 message。Mitigation：`FromError` 对未知 error 和 nil error 统一返回内部错误公开消息，仅在 `Cause` 中保留原始错误供日志、span 或 `errors.Is/As` 使用。
- [Risk] validation 既要保留字段明细又要迁移错误分类，容易丢失 `errors` 响应字段。Mitigation：保留 `ValidationFailedWithErrors` 或等价响应路径，单独测试 validation error 的 HTTP status、envelope code、message 和字段明细。
- [Risk] HTTP status 推导移动后 span 标注可能与响应 status 漂移。Mitigation：span error 标注复用 `common/http/response` 的同一状态码推导函数，并在 response 测试中覆盖内部错误和 validation error。

## Migration Plan

1. 在 `common/contract/errors` 定义语义模型、`Kind`、`Reason`、构造入口、`FromError` 与 `errors.Is/As` 行为，并移除 `HTTPStatus` 与 status 参数 API。
2. 在 `common/http/response` 增加唯一 HTTP status 推导函数，调整 `Fail`、`WriteError`、错误 helper、validation helper 和 span 标注。
3. 迁移 `common/http/binding`、`common/http/middleware`、`common/validation` 的旧错误构造和分类逻辑。
4. 按编译结果迁移 user-service 中调用共享错误构造器的 mapper 或 middleware 集成点，但不改变领域 sentinel error 和登录强制改密响应结构。
5. 更新并运行相关单元测试，最后运行 `go test ./common/...` 和 `make user-service-architecture-lint`。

回滚时恢复本 change 修改的共享错误模型、HTTP response 推导和调用点迁移；由于无数据库或部署资产变更，不需要数据回滚。

## Open Questions

无。
