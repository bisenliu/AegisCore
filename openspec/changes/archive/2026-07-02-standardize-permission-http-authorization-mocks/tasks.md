## 1. Mock 生成入口

- [x] 1.1 在 `user-service/internal/features/permission/transport/http/mock_generate.go` 增加 `authorization.Authorizer` 的 feature-local gomock 生成指令，使用独立 destination，避免覆盖现有 command/query controller mock。
- [x] 1.2 执行 `make user-service-generate`，提交生成的 permission HTTP authorization mock 测试文件，并检查生成物没有无关 drift。

## 2. 授权中间件测试改造

- [x] 2.1 改造 `user-service/internal/features/permission/transport/http/authorization_test.go`，删除 `fakeAuthorizer` 类型和所有手写调用计数断言。
- [x] 2.2 使用 gomock 改造授权通过、request context 用户 ID、拒绝、错误、invalid subject、RBAC negative 和 super admin wildcard decision 场景，精确验证 user id、Gin full path 和 HTTP method。
- [x] 2.3 使用 gomock 的未设置调用期望验证白名单和 `OPTIONS` bypass 场景不会调用 `authorization.Authorizer.Enforce`。
- [x] 2.4 保留真实 Gin engine 路由测试路径，确保缺失用户和非法用户 ID 场景仍返回未认证并且不会调用 authorizer。

## 3. 验证与收尾

- [x] 3.1 执行 `cd user-service && go test ./internal/features/permission/transport/http`，确认 permission HTTP 包测试通过。
- [x] 3.2 执行 `make user-service-architecture-lint`，确认 feature 边界和架构规则通过。
- [x] 3.3 执行生成 drift 检查，例如重新运行 `make user-service-generate` 后用 `git diff --exit-code -- user-service/internal/features/permission/transport/http` 确认 mockgen 无 drift。
- [x] 3.4 将本次预期代码、生成物和 OpenSpec artifacts 加到暂存区，再执行 `make lint`。
- [x] 3.5 在暂存本次预期变更后执行 `make verify`，确认最终验证通过且没有未暂存的预期 drift。
