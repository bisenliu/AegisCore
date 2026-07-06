## Context

`docs/TESTING.md` 已把 Go 测试断言规范化为优先使用 `testify/require`，`delivery-operations` 主规格也已要求使用语义化断言和受限的 `testing.T` 直接失败例外。`user-service/internal/router`、`providers` 和 `bootstrap` 是 user-service 运行时装配边界，覆盖健康检查、metrics、OpenAPI、pprof、Gin middleware、Fx provider、PostgreSQL/Redis/Ent provider、bootstrap validation 和 HTTP server lifecycle；这些测试仍保留大量历史 `t.Fatal`、`t.Fatalf` 和手写 if 断言。

本 change 只迁移测试断言风格，不改变生产代码行为。受影响路径限定为 `user-service/internal/router/**/*_test.go`、`user-service/internal/providers/**/*_test.go` 和 `user-service/internal/bootstrap/**/*_test.go`，实现时需要同时关注 runtime observability、delivery operations 与共享测试断言治理要求。

## Goals / Non-Goals

**Goals:**

- 将错误、对象、数值范围、集合、字符串、时间、panic、JSON 和正则检查迁移为 `require` 或必要时的 `assert` 语义化断言。
- 对 route 表、provider 输出、metrics family、日志字段、health check 结果等多个互相独立检查，使用 `assert` 收集独立失败。
- 消除可由 `require.ErrorContains`、`require.Len`、`require.Greater`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration`、`require.Panics` 等专属断言表达的 `require.True` / `require.False` 或手写 if。
- 对并发协调、channel handoff、goroutine 内结果传递或测试辅助工具边界中确实需要保留的直接失败调用，在 tasks 中列明残留原因。
- 保持路由注册、health、metrics、OpenAPI、pprof、Gin middleware、Fx provider、bootstrap validation 和 HTTP server lifecycle 生产语义不变。

**Non-Goals:**

- 不修改生产路由、provider 装配、HTTP server lifecycle、OpenAPI 生成物、metrics 输出格式、日志字段、tracing span 或配置 schema。
- 不迁移 `user-service/internal/features`、`user-service/cmd`、Ent schema、e2e、`common` 或部署资产测试。
- 不新增旧 metrics path、旧 pprof path、旧 route alias 兼容断言。
- 不新增旧断言兼容 helper、共享 wrapper、机械 `Fail*` 替换或仅服务于测试的生产 API。

## Decisions

1. 默认使用 `require` 迁移普通断言。

   理由：目标测试中多数手写断言检查初始化错误、HTTP status、响应结构、provider 构造结果、生命周期 hook 和依赖状态；阻塞式失败能避免前置条件失败后继续解引用或产生级联错误。备选方案是统一使用 `assert`，但不适合这些依赖前置条件的检查。

2. 对独立字段聚合使用 `assert`。

   理由：route 表、metrics family、日志字段和 health check 列表中常有多个互不依赖的期望值；`assert` 可以在单次测试中收集更多差异。备选方案是全部使用 `require`，但会降低这类 drift 的定位效率。

3. 使用专属断言替代泛化布尔表达式。

   理由：`require.True(strings.Contains(...))`、`len(...) == n`、`elapsed < timeout`、JSON 字符串比较或 panic/recover 手写控制流会丢失差异信息；`ErrorContains`、`Len`、`Less`、`JSONEq`、`Regexp`、`WithinDuration`、`Panics` 等断言能提供更明确的失败输出。备选方案是保留布尔断言，但不符合当前测试规范。

4. 保留少量并发和 goroutine 控制流例外。

   理由：HTTP server lifecycle 测试包含 handler 启停、channel handoff、blocked request 和 context cancellation；某些失败属于测试控制流而不是普通值断言。备选方案是追求零残留，但可能迫使测试引入不自然 helper 或隐藏并发路径。

5. 不跨出 issue 指定目录。

   理由：本次目标是 router、providers、bootstrap 历史测试断言迁移；feature、cmd、common 和 e2e 测试已有或应有独立治理。扩大范围会增加风险并混入不相关 diff。

## Risks / Trade-offs

- [Risk] 机械替换可能把需要继续执行的多字段诊断改为过早终止。→ Mitigation：对 route、metrics、日志字段和 health check 列表使用 `assert`，前置条件和依赖性检查继续使用 `require`。
- [Risk] 并发测试中直接在 goroutine 调用 `require` 可能与测试生命周期不匹配。→ Mitigation：goroutine 内优先向 channel 返回 error 或结果，在主 goroutine 断言；确需保留的控制流列入 tasks 例外。
- [Risk] HTTP server lifecycle 的时序断言迁移不当可能改变测试意图。→ Mitigation：保持现有 channel、context、listener 和 handler 编排，只替换断言表达。
- [Risk] import 调整可能产生未使用 import。→ Mitigation：迁移后运行 `gofmt` 和目标包 `go test`。
- [Risk] 残留扫描可能命中测试辅助工具中的预期错误构造或生产 helper 名称。→ Mitigation：只将 `t.Fatal` / `t.Error` / `Fail*` 调用视为残留断言，按 `docs/TESTING.md` 规则分类记录。

## Migration Plan

1. 使用 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/internal/router user-service/internal/providers user-service/internal/bootstrap --glob '*_test.go'` 建立迁移清单。
2. 分批迁移 router、providers、bootstrap 测试断言到 `require` / `assert`，删除不再使用的 import 并运行 `gofmt`。
3. 对剩余直接 `testing.T` 失败调用进行分类，确认符合 `docs/TESTING.md` 特殊例外并写入 `tasks.md`。
4. 运行残留扫描、`rg "github.com/stretchr/testify/(require|assert)" ...`、`go test ./user-service/internal/router ./user-service/internal/providers ./user-service/internal/bootstrap` 和 `openspec validate standardize-runtime-provider-test-assertions-no-compat`。

回滚策略：由于本 change 不修改生产代码、schema、API、OpenAPI 生成物或部署资产，如迁移导致测试语义不清，可按文件回退对应 `_test.go` 的断言迁移和 OpenSpec change 产物；无需运行数据迁移或提供运行时兼容步骤。

## Open Questions

- 无待决问题。实现阶段以 `docs/TESTING.md` 的例外规则、issue 验收扫描和目标包测试结果为准。
