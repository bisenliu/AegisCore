## 1. 共享 observability provider API

- [x] 1.1 修改 `common/runtime/observability/metrics/fx.go`，删除只包含无 tag `*config.Config` 的 `FxParams`，将 `NewFxProvider` 改为直接接收 `*config.Config`，并保留 nil config 错误和 runtime config 到 metrics `Options` 的转换逻辑。
- [x] 1.2 修改 `common/runtime/observability/tracing/fx.go`，删除只包含无 tag `fx.Lifecycle` 与 `*config.Config` 的 `FxParams`，将 `NewFxProvider` 改为直接接收 `fx.Lifecycle` 与 `*config.Config`，并保留 provider 构造错误传播和 `OnStop: provider.Shutdown` hook。
- [x] 1.3 更新所有 `commonmetrics.NewFxProvider` 与 `commontracing.NewFxProvider` 的直接调用点和编译引用，确保 `NewFxProvider` 不退化为 `NewProvider` 的无语义别名。

## 2. user-service Ent provider 依赖图

- [x] 2.1 修改 `user-service/internal/providers/ent.go`，删除 `NamedEntClientParams.Metrics` 与 `Tracing` 的 `optional:"true"` tag，让正式 Ent provider 依赖非 optional 的 `*commonmetrics.Provider` 与 `*commontracing.Provider`。
- [x] 2.2 确认 `user-service/internal/providers/fx.go` 继续注册 `commonmetrics.NewFxProvider` 和 `commontracing.NewFxProvider`，不新增服务私有 observability wrapper。
- [x] 2.3 保留或收紧 Ent observability nil fallback 的直接构造防御语义，但确保正式 `providers.Module` 不依赖 nil metrics/tracing 实现 disabled 降级。

## 3. 单元和 Fx graph 测试

- [x] 3.1 更新 metrics provider 测试，覆盖有效 runtime config 正常构造 provider、disabled 配置返回有效 provider，以及 nil config 返回错误。
- [x] 3.2 更新 tracing provider 测试，验证 provider 构造仍成功、构造错误仍传播、disabled 配置使用 no-op 或 `NeverSample` 语义，并验证 Fx lifecycle stop 仍调用 shutdown。
- [x] 3.3 更新 Ent provider 直接构造测试，显式提供 enabled/disabled metrics/tracing provider，并覆盖 nil fallback 仅作为直接构造防御语义存在。
- [x] 3.4 更新 Ent provider module/Fx graph 测试，验证缺失 metrics provider 时 graph 校验失败，缺失 tracing provider 时 graph 校验失败，enabled/disabled observability provider 均可成功构图。
- [x] 3.5 使用 Fx module/app 测试验证 user-service `providers.Module` 仍能解析并启动共享 metrics/tracing provider。

## 4. 规格和文档

- [x] 4.1 更新 `openspec/changes/standardize-observability-fx-dependencies/specs/shared-platform-primitives/spec.md`，确认只表达共享 runtime provider 输入治理和配置边界，不新增一次性 capability。
- [x] 4.2 更新 `openspec/changes/standardize-observability-fx-dependencies/specs/runtime-observability/spec.md`，确认只表达 provider composition、disabled/no-op、错误传播、shutdown lifecycle 和 Ent observability 必需依赖语义。
- [x] 4.3 如实现过程中发现 docs 与规格约束不一致，更新必要 docs，并保持影响范围不超出本 change 约定路径。

## 5. 验证

- [x] 5.1 在 `common` module 下运行 `go test -count=1 ./runtime/observability/metrics ./runtime/observability/tracing`。
- [x] 5.2 运行受影响的 user-service providers/bootstrap 测试，覆盖 Ent provider 和 `providers.Module` Fx dependency graph。
- [x] 5.3 运行 `make user-service-architecture-lint`。
- [x] 5.4 运行 `openspec validate standardize-observability-fx-dependencies`。
- [x] 5.5 暂存本 change 的预期代码、测试、规格和文档变更，检查 `git diff --cached` 只包含预期文件。
- [x] 5.6 运行 `make lint`，确认 lint 通过且没有生成物 drift。
- [x] 5.7 运行 `make verify`，确认生成物和 Fx dependency graph 没有 drift。
