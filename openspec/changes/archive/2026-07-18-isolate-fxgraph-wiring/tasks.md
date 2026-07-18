## 1. Module 拆分

- [x] 1.1 阅读 `user-service/internal/bootstrap/app.go`、`user-service/internal/providers/fx.go`、feature module 和 router/provider Invoke，标记 provider 注册与 runtime 激活边界。
- [x] 1.2 在 bootstrap 中引入无运行时激活的 `WiringModule`，仅保留 feature/provider wiring 和 HTTP/pprof server constructor 等构图所需 provider。
- [x] 1.3 在 bootstrap 中引入 `RuntimeModule`，承载 timezone 初始化、runtime dependency metrics、route 注册、RBAC lifecycle、HTTP/pprof server 激活和必须由 Invoke 驱动的 lifecycle。
- [x] 1.4 将正式 `AppModule` 改为组合 `WiringModule` 与 `RuntimeModule`，确认 `serve` 和现有测试继续使用正式完整 App module。

## 2. Fxgraph 命令

- [x] 2.1 调整 `user-service/cmd/fxgraph.go`，让 graph 命令使用 `WiringModule` 或专用无副作用 graph root，而不是完整生产 `AppModule`。
- [x] 2.2 如 wiring graph 丢失必要诊断边，新增服务侧专用 graph root，但不得注册真实 route、lifecycle hook、外部连接、后台资源或进程级初始化。
- [x] 2.3 保持 `fxgraph` 的公开 CLI 名称、flag、默认配置路径、退出码、输出文件语义和 DOT 非空输出不变。

## 3. Common Helper 边界

- [x] 3.1 检查 `common/runtime/fxgraph/fxgraph.go`，确认 helper 只处理传入 Fx option、稳定 DOT 输出和错误传播，不导入 user-service 私有配置或 feature 包。
- [x] 3.2 调整或新增 common fxgraph 测试，覆盖 helper 不自行要求服务私有配置、Ent、Redis、PostgreSQL、OTLP 或 HTTP server 输入。

## 4. 回归测试

- [x] 4.1 新增 user-service graph 测试，验证 graph 生成不调用 PostgreSQL、Redis 或 OTLP 构造/探测路径。
- [x] 4.2 新增 user-service graph 测试，验证 graph 生成不创建 workerpool、本地缓存或 tracing exporter 后台资源。
- [x] 4.3 新增 user-service graph 测试，验证 graph 生成不注册真实 route 或 runtime dependency metrics。
- [x] 4.4 新增 user-service graph 测试，保存并恢复 `TZ`、`time.Local` 和 Gin mode，验证 graph 生成不修改这些进程级状态。
- [x] 4.5 新增或调整正式 App 构图/启动相关测试，确认 `AppModule` 仍包含 runtime 激活链路，HTTP server、pprof server、routes、metrics、RBAC lifecycle 和 lifecycle hooks 不丢失。

## 5. 验证与收尾

- [x] 5.1 运行相关 Go 测试，例如 `go test ./...` 于受影响 module 或更精确 package，修复失败并保持断言语义化。
- [x] 5.2 运行 `make user-service-architecture-lint`，确认 module 拆分和测试未破坏架构边界。
- [x] 5.3 检查本次变更不涉及 OpenAPI、Ent schema、migration 或部署生成物；如出现相关 diff，说明原因或回退非预期生成物。
- [x] 5.4 将本次预期代码、测试、OpenSpec 和必要文档变更加到暂存区。
- [x] 5.5 运行 `make lint`，失败时修复后重跑并保持任务未完成直到通过。
- [x] 5.6 运行 `make verify`，失败时修复后重跑，并确认最终 drift 检查只包含已暂存的预期变更。
