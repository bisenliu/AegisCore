## Why

当前错误码体系已经能正确表达现有认证、授权、冲突、未找到和系统错误，`CodeConflict` 也已通过 `KindConflict` 正确映射为 `409 Conflict`。但错误码段分配和扩展规则尚未在共享契约中集中固化，后续新增 MFA、OAuth、设备管理、限流、配额或更多业务冲突错误时，容易出现跨段占用、语义混用或新增 `Kind` 后遗漏 HTTP 映射的问题。

## What Changes

- 在共享平台原语规格中明确应用错误码段分配规则，规定各段的语义边界、预留范围和扩展准入要求。
- 明确 `Code` 是稳定公开应用码，不等同于 HTTP status；HTTP status 继续只由低基数 `Kind` 推导。
- 明确新增错误码应优先复用既有 `Kind` 和稳定 `Reason`，不得按 feature 随意开段。
- 明确新增 `Kind` 时必须同步 `common/http/response.statusCode` 映射和响应测试，避免未知 `Kind` 降级为内部错误响应。
- 在 `common/contract/errors/code.go` 顶部补充错误码段说明，不改变任何现有错误码数值、响应 envelope 或 HTTP 映射行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 扩展跨服务应用错误契约，新增错误码段分配、预留范围和新增错误码准入规则。

## Impact

- 影响规格：`openspec/specs/shared-platform-primitives/spec.md` 的错误契约要求。
- 影响代码：`common/contract/errors/code.go` 的说明性注释。
- 不影响现有 API 响应结构、HTTP status、错误码数值、数据库 schema、OpenAPI 文档、部署资产或运行时依赖。
- 验证重点：`make user-service-architecture-lint`，以及相关 `common/contract/errors`、`common/http/response` 测试。
