## Context

`docs/TESTING.md` 已将测试断言与失败处理规范化为优先使用 `testify/require`，现有主规格也包含测试断言与失败处理风格要求。`common/runtime` 和 `common/security` 是跨服务共享能力的高复用边界，覆盖 config、datastore、id、localcache、logger、observability、rediskey、resources、scheduler、timezone、workerpool、auth、casbin、password 等包；这些包中的历史测试仍可能使用 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf` 或 `Fail*` 表达可语义化的普通断言。

本 change 只迁移测试断言风格，不改变生产代码行为。受影响路径限定为 `common/runtime/**/*_test.go` 和 `common/security/**/*_test.go`，实现时需要同时关注 shared runtime primitive、runtime observability、auth/password 安全原语和 Casbin shared wrapper 的测试覆盖。

## Goals / Non-Goals

**Goals:**

- 将常见错误、相等性、布尔条件、集合长度、nil、错误类型和状态检查迁移为语义化 `require` 断言。
- 对 metrics 或其他一次执行需收集多个独立字段失败的场景，按 `docs/TESTING.md` 使用 `assert`。
- 对并发协调、panic/recovery、benchmark 或测试框架边界中的特殊失败控制流，允许保留 `t.Fatal`、`t.Error` 或 `Fail*`，并在 tasks 验收中列明残留原因。
- 保持 runtime primitive、安全原语、metrics、tracing、logger、password KDF、JWT/auth 和 Casbin authorizer 的生产语义不变。

**Non-Goals:**

- 不迁移 `user-service` feature 测试。
- 不修改 Go 生产代码、HTTP API、数据库 schema、OpenAPI 生成物、部署清单、Prometheus/Grafana 资产或安全边界。
- 不新增兼容旧断言风格的 helper、wrapper 或双写断言。
- 不把手写失败判断机械替换为 `require.Fail`、`assert.Fail` 或等价 Fail helper。

## Decisions

1. 使用 `require` 作为默认迁移目标。

   理由：多数历史手写断言检查的是错误返回、对象状态、布尔条件或集合长度，阻塞式失败可以避免后续 nil dereference、错误状态级联和重复样板。备选方案是统一使用 `assert`，但这会在前置条件失败后继续执行，不符合当前测试规范的默认策略。

2. 仅在独立字段聚合诊断中使用 `assert`。

   理由：metrics family、label、counter 或统计快照等测试有时需要一次执行收集多个独立字段差异；此时 `assert` 能提供更完整诊断。备选方案是全部使用 `require`，但会降低同一采样结果中多字段 drift 的可定位性。

3. 保留无法语义化表达的特殊失败控制流。

   理由：并发测试、panic/recovery 验证、benchmark、goroutine 内测试控制流或测试框架边界可能需要显式 `t.Fatal`、`t.Error` 或 `Fail*` 表达停止、诊断或跨 goroutine 协调。备选方案是零残留，但可能迫使实现引入不自然的 helper 或降低诊断质量。

4. 通过扫描命令约束残留，而不是新增生产或测试兼容层。

   理由：本 change 的目标是测试风格统一，残留应通过 `rg` 与人工分类证明符合例外规则。备选方案是新增断言包装 helper，但会制造新的测试 API，偏离 `testify` 的语义化断言方式。

5. 不跨出 `common/runtime` 与 `common/security`。

   理由：用户明确限定迁移范围，且 `user-service` feature 测试已有独立 mock 与断言治理变更。备选方案是顺带迁移更多目录，但会扩大风险并混入不相关 diff。

## Risks / Trade-offs

- [Risk] 机械替换可能把需要继续执行的多字段诊断改成过早终止。→ Mitigation：对 metrics、collector、stats 类测试逐个判断是否需要 `assert`。
- [Risk] goroutine 内直接调用 `require` 可能与测试生命周期不匹配或产生竞态诊断。→ Mitigation：并发测试优先在主 goroutine 聚合结果后断言；确需保留 `t.Error` 的用法列入残留清单。
- [Risk] panic/recovery 测试可能因为迁移方式改变 defer/recover 控制流。→ Mitigation：保留控制流清晰的 `t.Fatal` 或使用 `require.Panics`、`require.NotPanics` 等语义化断言前先确认不改变测试意图。
- [Risk] 大范围测试文件 import 调整可能引入未使用 import。→ Mitigation：迁移后运行 `gofmt` 和目标包 `go test`。
- [Risk] 残留扫描命中可能包含外部生成或特殊 helper。→ Mitigation：仅扫描 `common/runtime` 和 `common/security` 的 `_test.go`，并在 tasks 中分类记录每个保留项。

## Migration Plan

1. 使用 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/runtime common/security --glob '*_test.go'` 建立迁移清单。
2. 按 package 迁移常见断言到 `require`，必要时补充 `assert`，并删除不再使用的 import。
3. 对保留的特殊失败控制流进行分类，确认上下文能体现保留原因，并在 `tasks.md` 验收项中列明。
4. 运行 `gofmt`、残留扫描、`rg "github.com/stretchr/testify/(require|assert)" common/runtime common/security --glob '*_test.go'`、`go test ./common/runtime/... ./common/security/...` 和 `openspec validate standardize-common-runtime-security-assertions-no-compat`。

回滚策略：如果迁移导致测试语义不清或失败，可按 package 回退对应 `_test.go` 的断言迁移；由于不涉及生产代码、schema、API 或部署资产，回滚不需要数据迁移或运行时兼容步骤。

## Open Questions

- 无待决问题。实现阶段以 `docs/TESTING.md` 的例外规则和本 change 的残留扫描验收为准。
