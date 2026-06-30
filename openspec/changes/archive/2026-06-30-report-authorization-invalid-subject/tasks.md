## 1. 应用层授权错误语义

- [x] 1.1 在 `user-service/internal/features/permission/application/authorization` 定义可通过 `errors.Is` 识别的非法 subject 错误。
- [x] 1.2 修改 `authorization.Enforce`，使 `uuid.Parse` 失败时返回 `false` 和明确错误，并保持不调用底层 `Engine`。
- [x] 1.3 更新 `authorization_test.go`，断言非法用户 ID 返回明确错误、`allowed=false` 且 engine 调用次数为 0。

## 2. HTTP 授权适配

- [x] 2.1 修改 `user-service/internal/features/permission/transport/http/authorization.go`，将非法 subject 错误映射为认证上下文无效路径，不再折叠为普通 `commoncasbin.ErrDenied`。
- [x] 2.2 更新 `authorization_test.go`，覆盖非法 UUID subject 的 HTTP 响应和 fake authorizer 调用语义。

## 3. 验证

- [x] 3.1 运行 `go test ./internal/features/permission/application/authorization ./internal/features/permission/transport/http`，确认授权相关单元测试通过。
- [x] 3.2 运行 `make user-service-architecture-lint`，确认 feature 边界和架构规则未被破坏。
- [x] 3.3 运行 `make lint`，确认代码风格和静态检查通过。
- [x] 3.4 运行 `make verify`，确认仓库级验证通过。
