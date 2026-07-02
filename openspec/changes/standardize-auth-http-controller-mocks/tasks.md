## 1. 生成入口与 mock 产物

- [x] 1.1 新增 `user-service/internal/features/auth/transport/http/mock_generate.go`，使用 `go tool mockgen` 为五个 auth command use case 生成 transport 测试包内 mock。
- [x] 1.2 执行 `make user-service-generate`，确认生成的 auth HTTP mock 文件位于 `user-service/internal/features/auth/transport/http`，且未进入全局 `mocks/` 目录或跨 feature mock 包。
- [x] 1.3 检查生成物 diff，确认 mockgen 命令、包名和接口列表与 user/permission HTTP controller 的生成风格一致。

## 2. controller 测试迁移

- [x] 2.1 改造 `user-service/internal/features/auth/transport/http/controller_test.go` 的测试 helper，使用 `gomock.NewController(t)` 和生成 mock 组装 `AuthControllerParams`。
- [x] 2.2 将登录测试迁移为 `gomock` expectation，覆盖用户名密码裁剪、`authctx.ClientContext` 注入、无效凭据、`password.ErrPasswordKDFBusy` 和普通服务错误映射。
- [x] 2.3 将改密测试迁移为 `gomock` expectation，覆盖 bearer token 归一化、新密码裁剪和 `identity.ErrUserNotFound` 映射。
- [x] 2.4 将 refresh 测试迁移为 `gomock` expectation，覆盖 refresh token 归一化、bearer-only token 拒绝和 `authdomain.ErrTokenInvalid` 映射。
- [x] 2.5 删除 `stubAuthUseCases`、其专用状态字段和只服务于该 stub 的 helper 参数类型，确保 `controller_test.go` 中不再出现 `stubAuthUseCases`。

## 3. 验证与收尾

- [x] 3.1 执行 `cd user-service && go test ./internal/features/auth/transport/http`，确认 auth HTTP controller 测试通过。
- [x] 3.2 再次执行 `make user-service-generate` 并检查生成物没有额外 drift。
- [x] 3.3 执行 `make user-service-architecture-lint`，确认架构边界和 OpenSpec 文档约束通过。
- [x] 3.4 将本次预期代码、生成物和 OpenSpec artifact 变更加到暂存区。
- [x] 3.5 执行 `make lint`，确认 lint 通过。
- [x] 3.6 执行 `make verify`，确认完整验证通过且最终 `git diff --exit-code` 未发现未处理 drift。
