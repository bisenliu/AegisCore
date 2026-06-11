# Rename ginvalidation to http binding

## What

将 `common/http/ginvalidation` 重命名为 `common/http/binding`，让共享 HTTP 请求绑定与校验适配层的包名更通用、更贴近职责。

包括：

- 目录从 `common/http/ginvalidation/` 迁移到 `common/http/binding/`。
- Go 包名从 `ginvalidation` 改为 `binding`。
- 更新 `common` 和 `user-service` 中所有 import、调用点和测试包名。
- 更新长期规则文档中对该能力的描述，使其引用 `common/http/binding`。
- 保留现有 `Bind`、`BindOrAbort`、`JSONBinder`、`StrictJSONBinder`、`JSONBinderWithOptions`、`URIBinder`、`QueryBinder` 和 `FormBinder` API 行为。

本变更只做命名迁移，不改变绑定、校验、错误归一化、错误响应、日志字段、HTTP status、响应 envelope 或 `common/validation` 核心逻辑。

## Why

`ginvalidation` 同时表达了 Gin 和 validation，但该包实际承担的是 HTTP transport 边界的请求绑定、绑定错误归一化、校验调用和失败响应适配。随着 `common/http` 下已有 middleware、response 等更通用命名，`binding` 更准确地表达该包对调用方提供的能力。

重命名后，feature controller 中的调用会更自然：

```go
binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder)
```

这能减少新协作者把该包误解为 `common/validation` 核心的一部分，也为未来 HTTP transport 侧扩展更多 binder 策略保留清晰命名空间。

## Scope

包括：

- 移动 `common/http/ginvalidation/binder.go` 到 `common/http/binding/binder.go`。
- 移动 `common/http/ginvalidation/validator.go` 到 `common/http/binding/validator.go`。
- 移动 `common/http/ginvalidation/validation_test.go` 到 `common/http/binding/validation_test.go`。
- 将上述文件的 `package ginvalidation` 改为 `package binding`。
- 将 `user-service/internal/features/user/transport/http/controller.go` 的 import 与调用从 `ginvalidation` 改为 `binding`。
- 将 `user-service/internal/features/auth/transport/http/controller.go` 的 import 与调用从 `ginvalidation` 改为 `binding`。
- 扫描并更新文档中仍作为当前规则或实施说明出现的 `common/http/ginvalidation` 引用。
- 保留历史 change 记录中的旧路径，除非该记录被当前长期规则文档引用为现行规范。

不包括：

- 不改变 `common/validation` 的 binder、validator、translation、field error 或 error classification 行为。
- 不改变任何 HTTP request DTO、controller command/query mapper 或 feature-local validation。
- 不改变失败响应 envelope、错误码、日志级别或日志字段。
- 不新增兼容 shim 包；本仓库内调用点一次性迁移到新包名。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不修改 Ent schema、migration、runtime config、Redis/PostgreSQL adapter 或业务 service。

## Acceptance Criteria

- `common/http/binding/` 存在，并包含原绑定/校验适配代码和测试。
- `common/http/ginvalidation/` 不再存在。
- Go 包名统一为 `binding`。
- 当前代码中不再存在需要迁移的 `ginvalidation` import、包名或调用点。
- `user-service` 的 user/auth controllers 使用 `github.com/aegiscore/common/http/binding`。
- 现有绑定、校验和错误响应测试语义保持不变。
- `common/` 中相关测试通过。
- `user-service/` 中受影响 controller/bootstrap HTTP 测试通过。
- `rg -n "ginvalidation" common user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md` 不应发现当前规则或代码引用。
