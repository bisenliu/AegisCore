## 1. 实现

- [x] 1.1 在 `user-services/internal/bootstrap/ent.go` 中调整 Ent clients 停止 lifecycle，确保 `user_db` 和 `common_db` close 都会被调用。
- [x] 1.2 为每个 Ent client close 错误添加具名上下文，并在多个 close 失败时返回合并错误而不是只返回第一个错误。
- [x] 1.3 保持 `ProvideEntClients` 的 Fx named injection、Ent client 创建数量和 bootstrap 包边界不变。

## 2. 测试与验证

- [x] 2.1 在 `user-services/internal/bootstrap` 增加或更新测试，覆盖两个 close 均失败时返回错误同时保留 `user_db` 与 `common_db` 上下文。
- [x] 2.2 覆盖单个 Ent client close 失败时返回具名错误、另一个 close 仍会执行的场景。
- [x] 2.3 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 2.4 在 `user-services/` 运行 `go test ./...` 验证用户服务模块。
