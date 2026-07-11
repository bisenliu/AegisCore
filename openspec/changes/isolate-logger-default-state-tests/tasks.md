## 1. 测试隔离实现

- [x] 1.1 审查 `common/runtime/logger` 与 `common/http/binding` 中 `logger.SetDefault` 的测试调用点，区分必须验证默认 logger 的用例和仅用于日志捕获的用例。
- [x] 1.2 在 `common/runtime/logger/logger_test.go` 中为必须替换默认 logger 的测试增加保存/恢复 helper，并确保这些测试不使用并行执行。
- [x] 1.3 将 `common/http/binding/validation_test.go` 的日志捕获测试改为通过 request context 注入局部 logger，不再调用 `logger.SetDefault`。

## 2. 验证

- [x] 2.1 运行 `openspec validate isolate-logger-default-state-tests`。
- [x] 2.2 在 `common` 模块下运行 `go test -race ./runtime/logger`。
- [x] 2.3 在 `common` 模块下运行 `go test -race ./http/binding`。
- [x] 2.4 运行 `make user-service-architecture-lint`。
- [x] 2.5 暂存本次预期变更后运行 `make lint`。
- [x] 2.6 暂存本次预期变更后运行 `make verify`。
