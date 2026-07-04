## 1. OpenAPI 转换工具测试与验证链

- [x] 1.1 将 `tools/openapi-convert/main.go` 拆分为可测试的 `run(ctx, args, stdout, stderr)` 执行函数，`main()` 仅负责传入 `os.Args` 并退出进程。
- [x] 1.2 使用 `flag.NewFlagSet` 替换全局 `flag` 解析，保持现有 CLI 参数、错误文案和成功输出语义不变。
- [x] 1.3 新增 `tools/openapi-convert` 单元测试，覆盖缺少必填参数、`root-path` 缺少 `root-server`、输入文件不存在和输出写入失败路径。
- [x] 1.4 新增 `tools/openapi-convert` 文件生成测试，使用 `t.TempDir()` 和最小 Swagger 2 输入断言 JSON、YAML、Go embed 输出文件与关键内容。
- [x] 1.5 为 `tools/openapi-convert` 增加模块级 `test` 入口，并在根 `Makefile` 中新增 `tools-openapi-convert-test`。
- [x] 1.6 调整根 `make test` 依赖，使其执行 `common-test`、`user-service-test` 和 `tools-openapi-convert-test`，并确认 `make verify` 自动覆盖该模块。

## 2. Role Infrastructure 关键路径覆盖

- [x] 2.1 为 `RolePermissionStore` 增加默认可执行测试，覆盖列表、删除、系统绑定补齐、空同步和映射 helper 路径。
- [x] 2.2 保留 `RolePermissionStore.Replace` 的 PostgreSQL 集成测试覆盖，不为 SQLite 默认测试修改生产 `FOR UPDATE` 锁语义。
- [x] 2.3 为 `RolePermissionStore.SyncSystemBindings` 增加默认可执行测试，覆盖新增、删除、保留绑定和新增/删除统计。
- [x] 2.4 为 `RolePermissionStore.SyncSystemBindings` 增加失败路径测试，覆盖缺失权限时不会返回完整成功结果且原绑定保持不变。
- [x] 2.5 运行 `go test -cover ./user-service/internal/features/role/infrastructure/postgres`，确认默认关键路径覆盖率达到 70%+。

## 3. Runtime 测试稳定性收敛

- [x] 3.1 将 `common/runtime/localcache` 中固定 `time.Sleep` 的 TTL 过期测试替换为 `require.Eventually` 或等价条件等待。
- [x] 3.2 将 `common/runtime/localcache` 并发回源测试中的调度 sleep 替换为通道、atomic 计数或 wait group 确认 goroutine 进入 loader 等待点。
- [x] 3.3 将 `common/runtime/workerpool` 测试中的固定 sleep 替换为通道信号或 `require.Eventually`，并保留明确失败诊断。
- [x] 3.4 将 `common/runtime/scheduler` 自动续租测试中的任务固定 sleep 替换为可观察锁续租条件或通道驱动的确定性等待。
- [x] 3.5 将 `common/runtime/timezone` 测试中的手动 `os.Setenv` 恢复替换为 `t.Setenv`，并继续用 `t.Cleanup` 隔离 `time.Local` 和包级状态。

## 4. Auth 测试时间确定性

- [x] 4.1 将 `user-service/internal/features/auth/infrastructure/redis` refresh session 上限裁剪测试中的循环 `time.Sleep` 替换为确定性 Redis score、可观察排序或局部测试 helper。
- [x] 4.2 将 `user-service/internal/features/auth/application/validators` token version 本地缓存过期测试替换为 `require.Eventually` 或等价条件等待，并保留真实 `localcache` 实例验证。
- [x] 4.3 运行 `rg "time\\.Sleep|os\\.Setenv\\(" --glob '*_test.go'`，确认本 change 目标范围内不再命中固定 sleep 或手动 env 恢复。

## 5. 验证与收尾

- [x] 5.1 运行 `go test -cover ./tools/openapi-convert/...`，确认工具模块测试通过且覆盖率不再为 0%。
- [x] 5.2 运行 `go test ./common/runtime/localcache ./common/runtime/workerpool ./common/runtime/scheduler ./common/runtime/timezone`，确认 runtime primitive 测试通过。
- [x] 5.3 运行 `go test ./user-service/internal/features/auth/application/validators ./user-service/internal/features/auth/infrastructure/redis`，确认 auth 相关测试通过。
- [x] 5.4 运行 `make user-service-architecture-lint`，确认 OpenSpec 中文和架构边界约束通过。
- [x] 5.5 暂存本次预期代码、测试、Makefile 和 OpenSpec artifact 变更。
- [x] 5.6 运行 `make lint`，失败时修复后重新运行，未通过不得标记完成。
- [x] 5.7 运行 `make verify`，确认完整验证通过且最终 drift 检查只暴露非预期变更或无 diff。
