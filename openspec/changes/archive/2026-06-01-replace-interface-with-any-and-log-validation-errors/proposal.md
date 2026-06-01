## Why

当前代码中仍存在 `interface{}` 空接口写法，与 Go 1.18+ 推荐的 `any` 别名不一致，降低代码风格统一性。同时 `common/validation` 的 `BindOrAbort` 在请求校验失败时使用 warning 级别日志，且字段级校验明细未进入结构化日志，不利于排查具体参数错误。

## What Changes

- 将项目中手写 Go 代码里的空接口类型 `interface{}` 统一替换为 `any`，保持 Go 1.26 代码风格一致。
- 调整 `common/validation/validation.go` 中 `BindOrAbort` 的无效请求日志等级为 error。
- 当校验错误包含字段级 errors 明细时，在 `BindOrAbort` 结构化日志中同时输出 `errors` 字段。
- 不改变 HTTP 响应 envelope、业务错误码、响应 message 或已有 validation 绑定/校验行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `request-validation`: 请求校验失败通过 `BindOrAbort` 输出日志时，必须使用 error 级别，并在存在字段级校验明细时记录 `errors` 字段。

## Impact

- 受影响代码主要包括 `common/validation/validation.go`，以及仓库内其他仍使用 `interface{}` 表示空接口的手写 Go 文件。
- 对外 HTTP API、响应 envelope、错误码、配置、数据库 schema 和依赖版本无兼容性影响。
- 日志可观察行为发生变化：无效请求日志将进入 error 级别输出，并包含字段级校验明细，便于线上排障。
