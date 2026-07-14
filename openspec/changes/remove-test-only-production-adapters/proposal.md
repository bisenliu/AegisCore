## Why

仓库已经通过多轮测试重构移除了全局 stub、手写 collaborator double 和部分 test hook，但当前仍有测试生成入口参与正常编译、生产函数为测试便利性保留 nil/default 分支、以及仅被同包测试调用的公开构造器。这些残留扩大了生产 API 和分支面，模糊了真实运行时契约，也使测试依赖隐式默认值或宽接口 fake，后续维护时容易把测试适配误当成业务兼容能力。

本 change 以主规格、生产调用图、生成链路和测试引用为依据完成一次仓库级清理：只有无真实运行时消费者、无稳定规格要求、由测试便利性引入且可由依赖注入、生成 mock 或 `_test.go` fixture 替代的代码才进入删除或收紧范围；真实业务边界、协议兼容、安全失败路径和运行时容错保持不变。

## What Changes

- 建立可重复的识别规则，扫描正常构建 Go 文件中的 test-only 命名、测试专用注释、仅测试引用的导出符号、nil/default 测试兜底、生成入口和宽接口 fake，并逐项以主规格和生产调用方复核。
- 将 23 个仅承载 `go:generate mockgen` 的 `mock_generate.go` 调整为 `generate` build tag 专用源文件，使 `go generate ./...` 继续发现生成指令，但正常构建和生产二进制不再编译这些测试生成入口。
- 收紧 `user-service/cmd/main.go` 的 CLI 依赖注入：删除 `rootCommandDependencies.withDefaults` 和 `runServe` 的 nil factory 兜底，要求生产入口与测试 fixture 都显式提供完整依赖；保留依赖注入本身以及现有 CLI command、flag、env、错误传播和生命周期行为。
- 将仅有测试消费者的 `common/runtime/workerpool.NewUnmanaged` 和 `user-service/internal/features/permission/infrastructure/redis.NewStoreWithInstance` 降为包内实现或由包内测试 fixture 直接组合，减少无稳定运行时职责的公开 API。
- 重构受影响测试：用完整 CLI dependency fixture 替代部分依赖默认补齐；在适合表达端口调用、失败注入和调用顺序的场景使用生成 mock；保留协议模拟器、并发记录器、真实轻量 adapter 和确定性执行 fixture 等具有明确测试语义的替身。
- 扩展架构/交付检查，阻止未带 `generate` build tag 的 `mock_generate.go`、明显 test-only 的生产 API/分支以及仅为测试便利性增加的兼容入口重新进入正式构建。
- 明确保留当前稳定行为：Casbin 授权白名单与 `OPTIONS` 旁路、refresh token 的 Bearer 输入兼容、request ID/metrics 等运行时 fallback、logger shutdown 容错、pprof handler 复用、HTTP response wrapper、Ent 观测的局部时间源和所有安全失败关闭边界均不在删除范围。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 收紧共享 runtime primitive 的公开构造器与正式构建边界，要求公开 API 具有真实运行时职责，并禁止仅为测试暴露的构造入口或测试适配分支。
- `delivery-operations`: 固化 mock 生成入口的 `generate` build tag、生成物 drift 校验和仓库级测试适配扫描规则。

## Impact

- 生产代码：`user-service/cmd/main.go`、`common/runtime/workerpool/pool.go`、`user-service/internal/features/permission/infrastructure/redis/store.go`，以及 `common/`、`user-service/` 下现有 `mock_generate.go`。
- 测试代码：`user-service/cmd/*_test.go`、`common/runtime/workerpool/*_test.go`、permission Redis watcher 测试，以及审计确认需要替换宽接口 fake/stub 的 auth/provider 测试 fixture 和 mock 生成物。
- 交付检查：`user-service/scripts/architecture-lint.sh` 及其 fixture/测试，必要时同步 `docs/TESTING.md` 的生成入口和测试替身规则。
- API 与运行时：不改变 HTTP API、OpenAPI、数据库 schema、Atlas migration、Redis key、Casbin policy、CLI 命令表面、部署资产或观测指标；`NewUnmanaged` 和 `NewStoreWithInstance` 属于仓库内无生产消费者的 Go API 收紧，不提供兼容别名。
- 安全：认证、授权、token version、refresh session、RBAC policy sync 和错误失败关闭语义必须通过现有核心测试保持不变。
