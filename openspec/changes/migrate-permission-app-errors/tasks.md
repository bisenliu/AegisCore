## 1. OpenSpec 与范围确认

- [x] 1.1 确认 `migrate-permission-app-errors` 的 proposal、design、`rbac-access-control` delta 和 `shared-platform-primitives` delta 已生成且可被 OpenSpec 识别。
- [x] 1.2 读取 apply 指令中的上下文文件，确认任务范围只覆盖 permission feature、共享错误契约消费方式和本 change artifacts。

## 2. 权限领域错误迁移

- [x] 2.1 修改 `user-service/internal/features/permission/domain/errors.go`，将 `ErrPermissionNotFound`、`ErrPermissionAlreadyExists`、`ErrPermissionInvalid` 和 `ErrSystemPermissionProtected` 定义为携带稳定 `Kind`、`Reason`、`Code`、中文公开 `Message` 的应用错误。
- [x] 2.2 调整 permission domain、command、query 或 infrastructure 测试，使权限错误断言覆盖直接返回和包装后的 `errors.Is` 语义。

## 3. 权限 HTTP 边界收敛

- [x] 3.1 修改 `user-service/internal/features/permission/transport/http/controller.go`，让权限业务 command/query 调用失败统一使用 `response.Fail(c, err)`。
- [x] 3.2 删除 `user-service/internal/features/permission/transport/http` 中仅用于权限错误翻译的 mapper 逻辑，确保不存在 `toPermissionHTTPError` 或等价兼容函数。
- [x] 3.3 检查 permission authorization HTTP transport 和测试，确保授权中间件不依赖权限目录错误 mapper，继续使用共享 response helper 渲染当前授权错误。
- [x] 3.4 更新 permission HTTP transport 测试，覆盖权限已存在返回 409、权限不存在返回 404、权限输入无效返回 400 validation、系统权限保护返回 409，且不依赖旧 mapper。

## 4. 验证与收尾

- [x] 4.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 4.2 运行 `rg "toPermissionHTTPError" user-service/internal/features/permission` 并确认无命中。
- [x] 4.3 运行 `go test ./user-service/internal/features/permission/...` 并确认通过。
- [x] 4.4 运行 `make user-service-architecture-lint` 验证 OpenSpec 和架构边界。
- [x] 4.5 将本次预期代码和 OpenSpec 变更加到暂存区，排除运行时文件 `AGENTS.md`、`CLAUDE.md`、`.multica/project/resources.json` 和 `.multica/**`。
- [x] 4.6 运行 `make lint` 和 `make verify`；如因本次范围外既有问题失败，记录失败命令和关键输出。
