## Why

`common/contract/response/failure.go` 当前承载的是 `BadRequest`、`ValidationFailed`、`Unauthenticated`、`Forbidden`、`Conflict`、`NotFound` 等语义化响应便利函数，而不是只描述失败响应模型本身。将文件重命名为 `helpers.go` 可以更准确表达其作为响应语义 helper 集合的职责，降低维护者按文件名定位代码时的认知成本。

## What Changes

- 将 `common/contract/response/failure.go` 重命名为 `common/contract/response/helpers.go`。
- 保持 Go package 仍为 `response`，导出的 helper 函数、错误构造函数、HTTP status、业务错误码、JSON 字段和公开 message 语义全部不变。
- 保持响应契约代码继续位于 `common/contract/response`，不新增包、不移动到服务模块、不修改 controller/service/repository 分层。
- 如有测试或文档引用具体文件名，则同步更新为 `helpers.go`；不要求调用方修改 import path 或函数调用方式。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `api-response-contract`: 明确标准语义响应 helper 可以由 `helpers.go` 承载，文件命名重构必须保持统一响应信封、错误码、HTTP status、错误映射和导出 API 兼容。

## Impact

- 影响代码：`common/contract/response/failure.go` 重命名为 `common/contract/response/helpers.go`。
- 影响 capability：`docs/opsx/CAPABILITY_MAP.md` 中的 `api-response-contract`，以及 `openspec/specs/api-response-contract/spec.md` 对响应 helper 组织方式的长期约束。
- API 兼容性：不改变 HTTP 路由、响应 JSON 字段、业务错误码、HTTP status 或公开 message。
- Go 兼容性：不改变 Go package 名、module path、导出函数名或调用方 import path。
- 数据兼容性：不涉及 Ent schema、Atlas migration、数据库结构或持久化数据。
- 依赖影响：不新增第三方依赖。
