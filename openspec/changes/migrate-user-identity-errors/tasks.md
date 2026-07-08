## 1. OpenSpec 与实现准备

- [x] 1.1 确认 `migrate-user-identity-errors` 的 proposal、design、`user-identity-management` delta 和 `shared-platform-primitives` delta 已生成且可被 OpenSpec 识别。
- [x] 1.2 读取 apply 指令中的上下文文件，确认任务范围只覆盖 shared identity、user feature 和本 change artifacts。

## 2. 用户身份错误迁移

- [x] 2.1 修改 `user-service/internal/shared/identity/errors.go`，将 `ErrUserNotFound` 与 `ErrUserAlreadyExists` 定义为携带 `Kind`、`Reason`、`Code`、`Message` 的应用错误，并保留 `errors.Is` 匹配语义。
- [x] 2.2 调整用户 command、query 或 infrastructure 测试，使用户身份错误断言覆盖直接返回和包装后的 `errors.Is` 语义。

## 3. 用户 HTTP 边界收敛

- [x] 3.1 修改 `user-service/internal/features/user/transport/http/controller.go`，让用户业务调用失败统一使用 `response.Fail(c, err)`。
- [x] 3.2 删除 `user-service/internal/features/user/transport/http` 中仅用于用户身份错误翻译的 mapper 逻辑，确保不存在 `toUserHTTPError` 或等价兼容函数。
- [x] 3.3 更新用户 HTTP transport 测试，覆盖用户已存在返回 409/冲突 code、用户不存在返回 404/未找到 code，且不依赖旧 mapper。

## 4. 验证与收尾

- [x] 4.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 4.2 运行 `go test ./user-service/internal/shared/identity/... ./user-service/internal/features/user/...` 并确认通过。
- [x] 4.3 运行 `make user-service-architecture-lint` 验证 OpenSpec 和架构边界。
- [x] 4.4 将本次预期代码和 OpenSpec 变更加到暂存区，排除运行时文件 `AGENTS.md`、`CLAUDE.md`、`.multica/project/resources.json` 和 `.multica/**`。
- [x] 4.5 运行 `make lint` 和 `make verify`；如因本次范围外既有问题失败，记录失败命令和关键输出。
