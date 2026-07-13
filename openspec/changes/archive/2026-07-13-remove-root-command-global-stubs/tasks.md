## 1. 命令依赖重构

- [x] 1.1 在 `user-service/cmd/main.go` 增加 root command 本地依赖结构和默认依赖构造。
- [x] 1.2 移除 `newLifecycleApp`、`runRBACSeed`、`runAssignSuperAdmin`、`runCreateSuperAdmin` package-level 可变函数变量。
- [x] 1.3 调整 `newRootCommand`、`main` 和 `runServe` 调用，使 `serve` 与 RBAC 子命令从本地依赖读取 factory/runner。

## 2. 测试改造

- [x] 2.1 更新 `TestRunServeStopContextPreservesUpstreamValuesWithoutCancellation`，通过局部 factory 测试 `runServe`，不再保存或恢复全局变量。
- [x] 2.2 更新 root command、RBAC seed、assign-super-admin、create-super-admin 测试，使每个测试构造本地 runner 替身。
- [x] 2.3 确认 `user-service/cmd/main_test.go` 不再对目标可变函数变量直接赋值，也不再通过 `t.Cleanup` 恢复这些变量。

## 3. 验证

- [x] 3.1 在 `user-service` 模块运行 `go test -race ./cmd` 并确认通过。
- [x] 3.2 在仓库根目录运行 `make user-service-architecture-lint` 并确认 OpenSpec artifacts 和架构约束通过。
- [x] 3.3 暂存本次预期代码和 OpenSpec 变更后运行 `make lint` 并确认通过。
- [x] 3.4 暂存本次预期代码和 OpenSpec 变更后运行 `make verify` 并确认通过，且忽略 Multica runtime 文件导致的工作区不干净噪声。
