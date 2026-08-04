## 1. 清理 watcher 测试构造边界

- [x] 1.1 删除 `watcher.go` 中只被测试调用的 `newWatcherWithMetrics`，保持 `NewWatcher` 到 `newWatcher` 的生产单一路径。
- [x] 1.2 在 `watcher_test.go` 增加 `newWatcherForTest` 并迁移 fake store/metrics 测试调用；真实 `*Store` 场景继续使用 `NewWatcher` 或生产内部核心构造。

## 2. 清理 RBAC baseline 与授权 wrapper

- [x] 2.1 删除 `rbacbaseline` 的未来默认角色注释模板、未消费 `permissionIDs` 及只验证该 helper 的测试，保留公开基线一致性覆盖。
- [x] 2.2 删除 permission HTTP transport 的白名单 rule/option alias 和 `WithAuthorizationWhitelist` wrapper，让 `Authorize` 与测试直接使用 common Casbin authorization 类型和 helper。
- [x] 2.3 对修改的 Go 文件运行 `gofmt`，检查不存在旧符号、兼容 alias、转发 wrapper、未来伪代码或新增生产分支。

## 3. 定向验证

- [x] 3.1 运行 permission Redis watcher、permission HTTP transport、rbacbaseline 和 common Casbin middleware 相关包测试。
- [x] 3.2 使用 Go 1.26 对 user-service 运行生产调用图 deadcode 检查，确认不再报告 `newWatcherWithMetrics`、`permissionIDs` 或 `WithAuthorizationWhitelist`，并单独保留 Ent 生成期及范围外测试支持入口的复核结论。
- [x] 3.3 运行 `openspec validate remove-rbac-test-only-production-code` 和 `make user-service-architecture-lint`。

## 4. 合并前门禁

- [x] 4.1 使用显式路径暂存本 change 的 OpenSpec artifacts、RBAC 代码和测试，检查暂存范围只包含预期变更。
- [x] 4.2 在预期变更全部暂存后运行 `make lint`；仅在通过后标记完成。
- [x] 4.3 在预期变更全部暂存后运行 `make verify`；仅在相关测试、生成检查和最终 diff 门禁全部通过后标记完成。
