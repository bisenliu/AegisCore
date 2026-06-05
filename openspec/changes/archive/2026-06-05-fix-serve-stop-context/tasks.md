## 1. Implementation

- [x] 1.1 在 `user-services/cmd/main.go` 中保留 `runServe` 入参的原始上游 context，并用其不可取消派生创建 Fx app stop root context。
- [x] 1.2 确保 stop context 继续通过 `fxAppStopTimeout` 创建独立停止预算，且不直接使用 signal-wrapped context 作为 stop root。
- [x] 1.3 如测试需要，在 `cmd` 包内引入最小 app factory seam，生产路径仍调用 `bootstrap.NewApp(configPath)`。

## 2. Tests

- [x] 2.1 增加 `runServe` 停止阶段测试，验证上游 context value 可被传递到 app stop context。
- [x] 2.2 增加 `runServe` 停止阶段测试，验证终止信号取消运行 context 后，app stop context 不会因该信号立即处于取消状态。
- [x] 2.3 验证 stop context 仍具有 `fxAppStopTimeout` 预算，并且 CLI 命令名、`--config` 参数和默认配置路径不变。

## 3. Validation

- [x] 3.1 对修改的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `user-services/` 执行 `go test ./...`。
- [x] 3.3 如工作区共享模块受影响，在 `common/` 执行 `go test ./...`。
