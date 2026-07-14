## 1. 配置模型

- [x] 1.1 在 `common/runtime/config` 增加 `RuntimeConfig`、`LifecycleConfig` 和 `runtime.lifecycle.start_timeout`、`runtime.lifecycle.stop_timeout` 字段。
- [x] 1.2 在默认配置中设置正数生命周期超时，并确保默认 `stop_timeout` 大于或等于默认 HTTP/gRPC shutdown timeout。
- [x] 1.3 更新配置 loader 默认值绑定，确保未声明新字段时自动使用默认值。
- [x] 1.4 更新配置校验，拒绝非正数 lifecycle timeout，并拒绝 `runtime.lifecycle.stop_timeout` 小于 HTTP 或 gRPC shutdown timeout 的配置。

## 2. Serve 生命周期接入

- [x] 2.1 调整 `user-service/cmd/serve.go`，在启动 Fx app 前读取配置中的 lifecycle timeout。
- [x] 2.2 移除 `user-service/cmd/serve.go` 中的 `fxAppStartTimeout` 和 `fxAppStopTimeout` 常量，避免 CLI 层保留 lifecycle 默认值。
- [x] 2.3 保留当前手动 `app.Start` / `app.Stop` 编排，并将 `Start` 和 `Stop` context timeout 从硬编码常量切换为配置值。
- [x] 2.4 保持停止阶段使用未被信号取消的上游 context，确保 context value 在 graceful shutdown 中继续可用。
- [x] 2.5 不新增 CLI flag，不改用 `fx.App.Run()`，不改变 HTTP、scheduler、workerpool 或业务处理逻辑。

## 3. 文档与测试

- [x] 3.1 更新 user-service 示例配置或开发文档，说明 `runtime.lifecycle.start_timeout` 与 `runtime.lifecycle.stop_timeout` 的含义和默认行为。
- [x] 3.2 更新 `common/runtime/config` 单元测试，覆盖默认值、显式配置、非正数配置和 stop timeout 小于协议 shutdown timeout 的错误。
- [x] 3.3 更新 `user-service/cmd` 单元测试，覆盖 serve 使用配置化 start/stop timeout、缺省配置路径不依赖 CLI 常量，并保持 stop context value 传递语义。
- [x] 3.4 删除或调整引用 `fxAppStartTimeout`、`fxAppStopTimeout` 的测试断言，改为断言配置默认值和 serve 消费配置值。
- [x] 3.5 确认本次 change 不需要 OpenAPI 生成、Ent 生成、Atlas migration 或部署观测资产生成。

## 4. 验证

- [x] 4.1 运行相关包测试，例如 `go test ./runtime/config` 于 `common` 模块和 `go test ./cmd` 于 `user-service` 模块。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认配置和 CLI 改动未破坏架构边界。
- [x] 4.3 将本次预期代码、文档和 OpenSpec 变更加到暂存区。
- [x] 4.4 运行 `make lint`，失败时修复后重新执行。
- [x] 4.5 运行 `make verify`，确认最终 drift 检查只暴露已预期并已暂存的变更。
