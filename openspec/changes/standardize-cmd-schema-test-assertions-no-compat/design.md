## Context

`docs/TESTING.md` 已把 Go 测试断言规范化为优先使用 `testify/require`，`delivery-operations` 主规格也已要求使用语义化断言和受限的 `testing.T` 直接失败例外。`user-service/cmd` 覆盖服务 root command、`serve` 与 RBAC 子命令，以及 flag/env 归一化、cleanup error 和命令帮助契约；`user-service/ent/schema` 覆盖 Ent schema field、edge、index、annotation、default、validator 和 mixin 结构。两个范围内仍保留历史手写断言和泛化布尔断言。

本 change 只迁移测试断言风格，不改变生产 CLI 或 Ent schema 行为。受影响路径限定为 `user-service/cmd/**/*_test.go`、`user-service/ent/schema/**/*_test.go` 和本 change 的 OpenSpec artifacts；实现时需要同时关注 delivery operations、RBAC CLI 和共享测试断言治理要求。

## Goals / Non-Goals

**Goals:**

- 将命令错误、flag/env 归一化、cleanup error、命令属性、schema field/index 数量、集合、字符串和 schema 元数据检查迁移为 `require` 或必要时的 `assert` 语义化断言。
- 对多个互相独立的 command property、schema field、index 或 annotation 检查，使用 `assert` 收集独立失败。
- 消除可由 `require.ErrorContains`、`require.Len`、`require.Greater`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp` 等专属断言表达的 `require.True` / `require.False` 或手写 if。
- 对确实属于特殊测试控制流、特殊诊断输出或测试辅助工具边界的直接 `testing.T` 失败调用，在 tasks 中列明残留原因。
- 保持服务前缀 Make target、CLI command contract、RBAC seed/admin 命令语义、Ent schema 和 Atlas migration 交付流程不变。

**Non-Goals:**

- 不修改 CLI 命令行为、Cobra command graph、flag/env 名称、cleanup 机制、RBAC seed 或超级管理员引导生产语义。
- 不修改 Ent schema、Ent 生成代码、Atlas migration、OpenAPI 生成物、部署资产或运行时配置。
- 不迁移 router/provider/bootstrap、feature、e2e 或 `common` 测试。
- 不新增旧 root command alias、旧 flag/env 名、无服务前缀 Make target 兼容断言。
- 不新增旧断言兼容 helper、共享 wrapper、机械 `Fail*` 替换或仅服务于测试的生产 API。

## Decisions

1. 默认使用 `require` 迁移依赖前置条件的断言。

   理由：cmd 测试中多数检查依赖 command 构造、执行错误、输出 buffer 或 cleanup 结果；Ent schema 测试中多数检查依赖字段或索引查找成功。前置失败后继续检查容易产生级联错误。备选方案是统一使用 `assert`，但不适合依赖前置条件的测试。

2. 对独立字段和 schema 列表检查使用 `assert`。

   理由：同一 command 的多个 property、同一 schema 的多个 field/index 元数据通常互相独立；`assert` 可以在单次测试中展示更多 drift。备选方案是全部使用 `require`，但会降低 schema 结构差异定位效率。

3. 使用专属断言替代泛化布尔表达式。

   理由：`len(...) == n`、`strings.Contains(...)`、集合包含、正则匹配、JSON 字符串比较和错误消息检查都已有更具体断言，能提供更清晰失败信息。备选方案是保留 `True` / `False`，但不符合当前测试规范。

4. 不通过测试迁移引入兼容行为。

   理由：本次目标是断言表达一致性，不是恢复旧 CLI path、旧 env key、旧 Make target 或旧 schema 形态。新增兼容断言会掩盖当前稳定契约。备选方案是在测试中同时接受新旧形态，但会削弱交付约束。

5. 不跨出 issue 指定目录。

   理由：router/provider/bootstrap、feature、common 和 e2e 测试已有或应有独立治理。扩大范围会增加风险并混入不相关 diff。

## Risks / Trade-offs

- [Risk] 机械迁移可能把需要收集多个字段 drift 的 schema 检查改为过早终止。→ Mitigation：字段、索引和 annotation 的独立检查按需使用 `assert`，查找失败或后续依赖检查继续使用 `require`。
- [Risk] CLI command 测试中输出和错误检查迁移不当可能隐藏命令契约变化。→ Mitigation：保持现有 command 构造和执行路径，只替换断言表达，并用 `ErrorContains`、`Contains`、`Regexp` 等专属断言表达输出预期。
- [Risk] Ent schema 测试 import 调整可能产生未使用 import。→ Mitigation：迁移后运行 `gofmt` 和目标包 `go test`。
- [Risk] 残留扫描可能命中测试辅助工具中的特殊控制流。→ Mitigation：逐项分类，只有符合 `docs/TESTING.md` 例外规则的命中才保留并写入 `tasks.md`。

## Migration Plan

1. 使用 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/cmd user-service/ent/schema --glob '*_test.go'` 建立迁移清单。
2. 分批迁移 `user-service/cmd` 与 `user-service/ent/schema` 测试断言到 `require` / `assert`，删除不再使用的 import 并运行 `gofmt`。
3. 对剩余直接 `testing.T` 失败调用进行分类，确认符合 `docs/TESTING.md` 特殊例外并写入 `tasks.md`。
4. 运行残留扫描、`rg "github.com/stretchr/testify/(require|assert)" user-service/cmd user-service/ent/schema --glob '*_test.go'`、`go test ./user-service/cmd ./user-service/ent/schema` 和 `openspec validate standardize-cmd-schema-test-assertions-no-compat`。

回滚策略：由于本 change 不修改生产代码、schema、API、OpenAPI 生成物或部署资产，如迁移导致测试语义不清，可按文件回退对应 `_test.go` 的断言迁移和 OpenSpec change 产物；无需运行数据迁移或提供运行时兼容步骤。

## Open Questions

- 无待决问题。实现阶段以 `docs/TESTING.md` 的例外规则、issue 验收扫描和目标包测试结果为准。
