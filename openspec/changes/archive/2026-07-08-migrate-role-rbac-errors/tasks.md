## 1. OpenSpec 与实现准备

- [x] 1.1 确认 `migrate-role-rbac-errors` 的 proposal、design、`rbac-access-control` delta 和 `shared-platform-primitives` delta 已生成且可被 OpenSpec 识别。
- [x] 1.2 读取 apply 指令中的上下文文件，确认任务范围只覆盖 role feature、共享错误契约消费方式和本 change artifacts。

## 2. 角色领域错误迁移

- [x] 2.1 修改 `user-service/internal/features/role/domain/errors.go`，将角色目录、用户角色绑定和角色权限绑定错误定义为携带稳定 `Kind`、`Reason`、`Code`、中文公开 `Message` 的应用错误。
- [x] 2.2 调整 role domain、command、query、seed 或 infrastructure 测试，使角色与绑定错误断言覆盖直接返回和包装后的 `errors.Is` 语义。
- [x] 2.3 检查 role 对 `identity.ErrUserNotFound` 与 `permissiondomain.ErrPermissionNotFound` 的消费路径，确保这些跨 feature 应用错误被返回或包装后仍可由共享 response helper 渲染。

## 3. 角色 HTTP 边界收敛

- [x] 3.1 修改 `user-service/internal/features/role/transport/http/role_controller.go`，让角色目录 command/query 调用失败统一使用 `response.Fail(c, err)`。
- [x] 3.2 修改 `user-service/internal/features/role/transport/http/user_role_controller.go`，让用户角色 command/query 调用失败统一使用 `response.Fail(c, err)`。
- [x] 3.3 修改 `user-service/internal/features/role/transport/http/role_permission_controller.go`，让角色权限 command/query 调用失败统一使用 `response.Fail(c, err)`。
- [x] 3.4 删除 `user-service/internal/features/role/transport/http` 中仅用于角色错误翻译的 mapper 逻辑，确保不存在 `toRoleHTTPError` 或等价兼容函数。
- [x] 3.5 更新 role HTTP transport 测试，覆盖角色已存在返回 409、系统角色保护返回 409、角色停用返回 409、用户角色或角色权限绑定已存在返回 409、角色/用户/权限/绑定不存在返回 404、角色输入无效返回 400 validation，且不依赖旧 mapper。

## 4. 验证与收尾

- [x] 4.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 4.2 运行 `rg "toRoleHTTPError" user-service/internal/features/role` 并确认无命中。
- [x] 4.3 运行 `go test ./user-service/internal/features/role/...` 并确认通过。
- [x] 4.4 运行 `make user-service-architecture-lint` 验证 OpenSpec 和架构边界。
- [x] 4.5 将本次预期代码和 OpenSpec 变更加到暂存区，排除运行时文件 `AGENTS.md`、`CLAUDE.md`、`.multica/project/resources.json` 和 `.multica/**`。
- [x] 4.6 运行 `make lint` 和 `make verify`；如因本次范围外既有问题失败，记录失败命令和关键输出。
