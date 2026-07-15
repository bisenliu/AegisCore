## Why

user-service 的 HTTP 或 pprof server 非预期退出时，bootstrap 虽会调用 `fx.Shutdowner`，但 `serve` 命令只等待外部 context，无法观察 `App.Wait()` 的 shutdown signal；因此 App 和 CLI 进程可能继续运行，内部故障也不能稳定转换为非零进程退出码。需要统一外部信号和内部故障的退出协调语义，使运行时故障能够立即触发完整、有限时且仅执行一次的停止流程。

## What Changes

- 扩展 `serve` 命令使用的最小 App 生命周期接口，使其同时等待外部 context 与 `App.Wait()` 返回的 `fx.ShutdownSignal`。
- 统一所有退出来源进入单次、带停止预算的 `App.Stop()`，并保留手动 `Start`/`Stop` 组装方式及其可测试性。
- 将内部 shutdown signal 的非零 exit code 转换为非零 Cobra error，由现有 main 入口负责最终进程退出码；外部 `SIGINT`/`SIGTERM` 仍正常退出。
- HTTP 或 pprof server 非预期退出时调用 `Shutdown(fx.ExitCode(1))`，正常 server 关闭不触发故障退出。
- 增加外部信号、内部 shutdown、非零 exit code、Stop error 和并发退出竞争的命令层及 bootstrap 回归测试。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `runtime-observability`: 明确 HTTP 与 pprof server 非预期退出必须发出带非零 exit code 的 Fx shutdown signal，且正常关闭不得误报为内部故障。
- `delivery-operations`: 明确 user-service CLI 必须把内部 shutdown signal 串联到单次优雅停止和非零 Cobra error，同时保持外部终止信号正常退出。

## Impact

- 受影响代码：`user-service/cmd/serve.go`、`user-service/internal/bootstrap/server.go`、`user-service/internal/bootstrap/pprof.go` 及对应测试。
- 受影响规格：`runtime-observability` 与 `delivery-operations` 的稳定退出和交付行为。
- HTTP/pprof endpoint、监听地址、业务 API、认证与 RBAC 安全边界均不变化；不影响数据库 schema、Atlas migration、OpenAPI 生成物、`common/` 契约或依赖版本。
- 不修改 Docker、Compose、Kubernetes、Helm、Prometheus 或 Grafana 资产，也不调整终止宽限期；发布和回滚仍使用现有 user-service 构建与部署流程。
