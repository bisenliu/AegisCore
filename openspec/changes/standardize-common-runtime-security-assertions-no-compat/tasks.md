## 1. 迁移清单

- [x] 1.1 执行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/runtime common/security --glob '*_test.go'`，建立 `common/runtime` 和 `common/security` 历史失败控制流清单。
- [x] 1.2 按包分类清单，区分可迁移的常见业务断言、需要 `assert` 聚合诊断的独立字段检查、以及符合 `docs/TESTING.md` 例外规则的并发、panic/recovery、benchmark 或测试框架边界用法。

## 2. common/runtime 测试迁移

- [x] 2.1 迁移 `common/runtime/config`、`common/runtime/datastore`、`common/runtime/id`、`common/runtime/rediskey`、`common/runtime/resources` 和 `common/runtime/timezone` 测试中的常见断言到 `testify/require`。
- [x] 2.2 迁移 `common/runtime/localcache` 测试断言，保留必要的并发协调或 panic/recovery 特殊控制流，并确保缓存容量、TTL、singleflight、stats 和关闭语义不变。
- [x] 2.3 迁移 `common/runtime/logger` 和 `common/runtime/observability` 测试断言；metrics 多字段结构化校验如需收集多个独立失败，按 `docs/TESTING.md` 使用 `testify/assert`。
- [x] 2.4 迁移 `common/runtime/scheduler` 和 `common/runtime/workerpool` 测试断言，保留必要的 goroutine、panic/recovery、benchmark 或生命周期控制流例外。

## 3. common/security 测试迁移

- [x] 3.1 迁移 `common/security/auth` 测试断言到 `testify/require`，确保 JWT claims、subject、`jti`、token version、过期和错误路径语义不变。
- [x] 3.2 迁移 `common/security/password` 测试断言到 `testify/require`，确保 Argon2id 参数、hash 编码、验证结果、资源预算和 `ErrPasswordKDFBusy` 语义不变。
- [x] 3.3 迁移 `common/security/casbin` 测试断言到 `testify/require`，确保 `Enforce`、`Authorizer.Authorize`、`ErrNotConfigured` 和 `ErrDenied` 语义不变。

## 4. 残留例外和格式化

- [x] 4.1 对所有修改过的 Go 测试文件运行 `gofmt`。
- [x] 4.2 重新执行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/runtime common/security --glob '*_test.go'`，确认剩余命中均符合 `docs/TESTING.md` 特殊例外规则。
- [x] 4.3 在本任务文件中记录剩余命中清单和保留原因；当前待实施后填写。
- [x] 4.4 执行 `rg "github.com/stretchr/testify/(require|assert)" common/runtime common/security --glob '*_test.go'`，确认迁移后的实际使用点存在。

## 5. 验证

- [x] 5.1 执行 `go test ./common/runtime/... ./common/security/...` 并修复失败。
- [x] 5.2 执行 `openspec validate standardize-common-runtime-security-assertions-no-compat` 并修复失败。
- [x] 5.3 检查 `git diff -- common/runtime common/security openspec/changes/standardize-common-runtime-security-assertions-no-compat`，确认只包含本 change 预期代码和 OpenSpec artifact 变更。
- [x] 5.4 在完成实现、规格和文档任务后，先暂存本次预期变更，再执行 `make lint` 和 `make verify`；任一命令未通过时不得将 change 标记为完成。

## 6. 残留例外清单

- [x] 6.1 剩余命中及保留原因如下：
  - `common/runtime/observability/metrics/runtime_dependencies_test.go`: `redis ping did not start` 和 `GatherContext did not finish after request context cancellation` 位于 `select` 超时分支，用于验证 Redis metrics scrape context 取消和 goroutine 完成的测试控制流。
  - `common/runtime/scheduler/scheduler_test.go`: `local gate was not released after global concurrency skip` 和 `task did not observe cancellation after renew failure` 位于并发/取消路径的 `select` 或非阻塞检查分支，用于验证 scheduler gate 释放和 renew failure 取消控制流。
  - `common/runtime/workerpool/pool_test.go`: `Submit did not return after worker became available`、`task did not observe submit context cancellation`、`Stop did not return` 和 `blocked Submit did not return` 位于 `select` 超时分支，用于验证 workerpool goroutine 协调和停止行为。
  - `common/runtime/workerpool/pool_test.go`: `waitForCount` 与 `waitForPool` 中的 `t.Fatalf` 是测试 helper 的轮询/等待超时诊断，保留用于输出当前等待进度或 pool stats。
