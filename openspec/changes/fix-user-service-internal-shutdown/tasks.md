## 1. 命令层退出协调

- [x] 1.1 修改 `user-service/cmd/serve.go` 的最小 `lifecycleApp` 接口，加入 `Wait() <-chan fx.ShutdownSignal`，并让成功启动后的等待逻辑同时消费外部 context 与 Fx shutdown signal。
- [x] 1.2 将所有成功启动后的退出来源汇聚到一次使用 `runtime.lifecycle.stop_timeout` 的 `App.Stop()`，保留未取消上游 context value，并实现内部非零 exit code、Stop error 及两者并存时的 Cobra error 语义。
- [x] 1.3 更新 `user-service/cmd` 的局部 lifecycle App 测试替身，补充外部正常退出、内部零/非零 exit code、Stop error 和并发退出竞争测试，断言 Stop 只执行一次且不会死锁。

## 2. Bootstrap 内部故障信号

- [x] 2.1 修改 `user-service/internal/bootstrap/server.go`，使 HTTP `Serve` 非预期退出调用 `Shutdown(fx.ExitCode(1))`，同时保留正常 lifecycle 关闭错误过滤与 shutdown 请求失败日志。
- [x] 2.2 修改 `user-service/internal/bootstrap/pprof.go`，使 pprof 非预期 listener/server 退出调用 `Shutdown(fx.ExitCode(1))`，并准确区分正常 `Server.Shutdown` 与意外 listener 关闭。
- [x] 2.3 更新 `user-service/internal/bootstrap/http_test.go` 和 `pprof_test.go`，断言非预期故障携带 exit code `1`、正常关闭不触发失败 signal、Shutdown error 仍可诊断。

## 3. 定向验证与规格检查

- [x] 3.1 对本 change 修改的 Go 文件执行 `gofmt`，然后运行 `cd user-service && go test ./cmd ./internal/bootstrap -count=1`，确认命令层与 bootstrap 回归测试通过。
- [x] 3.2 核对 proposal、design、`runtime-observability` 与 `delivery-operations` delta specs 和实现一致，并运行 `openspec validate fix-user-service-internal-shutdown`。
- [x] 3.3 运行 `make user-service-architecture-lint`，确认生命周期代码归属、中文文档和架构边界检查通过。

## 4. 最终交付门禁

- [x] 4.1 在实现、测试、规格和文档任务全部完成后，使用 `git add` 仅暂存 `user-service/cmd/`、`user-service/internal/bootstrap/` 与 `openspec/changes/fix-user-service-internal-shutdown/` 中本 change 的预期变更，并用 `git diff --cached --check` 和 `git diff --cached --name-only` 检查暂存范围。
- [x] 4.2 在预期变更已暂存后运行 `make lint`；未通过时修复并重新暂存，未通过前不得勾选本任务。
- [x] 4.3 在预期变更已暂存且 lint 通过后运行 `make verify`，通过其最终 `git diff --exit-code` 暴露生成物 drift 或未暂存意外变更；未通过时修复并重新执行，未通过前不得将 change 视为完成。
