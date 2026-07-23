## 1. Bootstrap 实现

- [x] 1.1 在 `user-service/internal/bootstrap/app.go` 新增未导出的 `registerRuntimeServers(_ *http.Server, _ *PprofServer)`，函数注释说明其用于强制解析 runtime server 并触发 lifecycle hook 注册。
- [x] 1.2 将 `RuntimeModule` 中两个空匿名 server `fx.Invoke` 替换为 `fx.Invoke(registerRuntimeServers)`，保留现有模块顺序和 Fx 分类注释语义。

## 2. 测试更新

- [x] 2.1 更新 `user-service/internal/bootstrap/app_test.go` 或 `validation_test.go`，验证 `AppModule` 仍可通过完整 runtime graph 校验。
- [x] 2.2 补充源码级测试，断言 `RuntimeModule` 使用 `registerRuntimeServers`，并避免回退到 `func(*http.Server) {}` 或 `func(*PprofServer) {}` 这类空匿名 Invoke。

## 3. 验证与交付

- [x] 3.1 运行 `go test ./user-service/internal/bootstrap`，确认 bootstrap 包测试通过。
- [x] 3.2 确认本次变更不需要 OpenAPI 生成、Ent 生成、Atlas migration 或部署观测资产生成。
- [x] 3.3 暂存本次预期代码、OpenSpec 和文档变更后运行 `make lint`。
- [x] 3.4 在暂存本次预期变更后运行 `make verify`，确认完整验证和 drift 检查通过。
