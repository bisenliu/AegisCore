## 1. 测试实现

- [x] 1.1 在 `common/http/response/response_test.go` 中补充可复用响应 envelope 解码和字段断言 helper，保持断言使用语义化 `require`。
- [x] 1.2 为 `Created` 和 `NoContent` 增加直接单元测试，覆盖 `201` 成功 envelope、`CodeOK`、`MessageCreated`、`data` 以及 `204` 空 body。
- [x] 1.3 为 `ValidationFailed`、`Unauthenticated`、`Forbidden`、`Conflict`、`NotFound` 增加直接单元测试，覆盖当前 HTTP status、应用错误码和公开 message。

## 2. 验证

- [x] 2.1 运行 `openspec validate cover-response-helpers-no-compat`，确认 change artifacts 合法。
- [x] 2.2 运行 `go test -cover ./common/http/response` 和 `go test -coverprofile=<profile> ./common/http/response`，再用 `go tool cover -func <profile>` 确认目标 wrapper 均已覆盖。
- [x] 2.3 暂存本次预期变更后运行 `make lint` 和 `make verify`；如因环境或非本次变更阻塞，记录阻塞原因和已完成的替代验证。
