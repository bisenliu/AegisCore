## Why

`common/contract/errors` 当前把应用错误与 HTTP status 直接绑定，导致跨服务错误契约承担了传输层渲染责任，也让非 HTTP 调用方继承不必要的状态码语义。现在需要把共享错误模型重塑为语义驱动的应用错误，由稳定 `Kind` 表达错误类别、`Reason` 表达业务原因，并让 HTTP 层成为唯一状态码推导位置。

## What Changes

- **BREAKING**：移除 `common/contract/errors.Error` 中的 `HTTPStatus` 字段，移除旧状态码直连模型、旧 factory API 和兼容适配路径。
- **BREAKING**：将共享错误构造 API 改为接收或派生 `Kind`、`Reason`、`Code`、公开 `Message` 与可选 `Cause`，不再由 `common/contract/errors` 保存 HTTP status。
- 引入稳定 `Kind` 与 `Reason`：`Kind` 表达通用错误类别并作为 HTTP status 推导输入；`Reason` 表达业务中立或业务边界声明的具体原因。
- 调整 `FromError`、`errors.Is/As` 支持和测试，确保 nil error、未知 error、wrapped application error、validation error 与内部错误脱敏有明确行为。
- 调整 `common/http/response`，由 HTTP response helper 根据 `Kind` 推导 HTTP status 并输出统一 response envelope。
- 同步调整 `common/http/binding`、`common/http/middleware`、`common/validation` 中依赖旧错误构造方式的代码。
- 保留 feature 层已有 `toXHTTPError` mapper 的迁移边界，本变更不迁移 user/auth/role/permission 的领域 sentinel error，不调整登录强制改密响应结构。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 重塑共享应用错误模型和 HTTP 错误渲染责任，要求 `common/contract/errors` 不再暴露 HTTP status，由 `common/http/response` 基于错误 `Kind` 统一推导 HTTP status。

## Impact

- 影响共享契约代码：`common/contract/errors` 的核心类型、构造函数、错误转换、包装与单元测试。
- 影响 HTTP helper：`common/http/response` 的错误写入、状态码推导、span 错误标注和响应测试。
- 影响共享 HTTP/validation 消费方：`common/http/binding`、`common/http/middleware`、`common/validation` 中创建或分类共享错误的路径。
- 影响 API 运行时错误响应状态码来源，但 response envelope 的稳定字段仍由共享契约提供。
- 不涉及数据库 schema、migration、部署资产或 OpenAPI 生成物；若代码注解未改变，不需要重新生成 OpenAPI。
- 验证重点为 `openspec validate reshape-shared-error-contract`、`go test ./common/...` 和 `make user-service-architecture-lint`。
