## 1. 迁移 Ent Provider

- [x] 1.1 在 `user-services/internal/bootstrap/` 新增 Ent client provider 文件，迁移 `ClientParams`、`NamedClients`、`NewNamedClients` 和内部 client 构造逻辑。
- [x] 1.2 更新 `UserServiceModule`，将 provider 引用从 `entclient.NewNamedClients` 改为 `NewNamedClients`，并移除 `internal/entclient` import。
- [x] 1.3 删除 `user-services/internal/entclient/provider.go` 和空的 `user-services/internal/entclient/` 包目录。

## 2. 测试与引用清理

- [x] 2.1 搜索并清理 `user-services/internal/entclient`、`entclient.NewNamedClients` 等旧包引用。
- [x] 2.2 添加或调整 `bootstrap` 包测试，验证 `NewNamedClients` 提供具名 `user_db` 和 `common_db` Ent clients，并注册停止关闭 hook。
- [x] 2.3 确认现有 Fx validation 测试仍覆盖 `UserServiceModule` 依赖图解析和 repository 获取具名 `user_db` Ent client。

## 3. 验证

- [x] 3.1 对修改过的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `user-services/` 运行 `go test ./...`。
- [x] 3.3 在 `common/` 运行 `go test ./...`，确认共享基础设施能力未被破坏。
- [x] 3.4 运行 `openspec status --change "merge-entclient-into-bootstrap"`，确认变更 artifacts 和任务状态可被 apply 阶段识别。
