## Context

`common/http` 是跨服务共享 HTTP helper 的稳定边界，测试覆盖 binding、middleware、OpenAPI、pprof 和 response 等行为。当前项目测试说明已经要求优先使用 `testify/require` 和语义化断言，但 `common/http` 历史测试仍可能保留手写 `t.Fatal*`、`t.Error*` 或非语义化 `Fail*` 风格，导致失败输出不一致，也容易在后续迁移中把手写判断机械替换为不可读的失败方法。

本变更只治理测试实现和测试规范，不改变 `common/http` 生产代码、HTTP response envelope、middleware 语义、OpenAPI helper 输出、pprof route 行为、数据库 schema、部署清单或 user-service HTTP transport 测试。

## Goals / Non-Goals

**Goals:**

- 将 `common/http/**/*_test.go` 的 HTTP status、header、JSON envelope、OpenAPI 输出、pprof route 和 binding 错误断言迁移到语义化 `testify/require`。
- 在确需一次性收集多个独立响应字段失败的测试中允许使用 `testify/assert`，但前置条件和依赖性检查仍使用 `require`。
- 避免新增 `require.Fail*`、`assert.Fail*` 或项目自定义兼容 helper 来模拟旧断言风格。
- 在 tasks 中记录 `rg` 扫描后仍保留的特殊例外，并说明其符合 `docs/TESTING.md` 的原因。
- 通过 `go test ./common/http/...` 和 `openspec validate standardize-common-http-test-assertions-no-compat` 验证迁移。

**Non-Goals:**

- 不修改 `common/http` 生产行为、HTTP response envelope、CORS/middleware 语义、OpenAPI helper 行为或 pprof route 行为。
- 不迁移 `user-service` HTTP transport 测试。
- 不新增旧 header、旧 envelope 或旧 CORS 行为兼容断言。
- 不新增旧断言风格兼容 helper、双写断言或测试专用生产分支。

## Decisions

1. 使用 `require` 作为默认断言库。

   原因：`common/http` 测试多数后续检查依赖绑定结果、响应对象或 JSON 解析结果。阻塞式失败可以减少级联错误，并与 `docs/TESTING.md` 保持一致。

   备选方案：统一使用 `assert` 收集更多失败。该方案会在初始化失败或响应对象不满足前置条件时产生噪声，不适合作为默认策略。

2. 使用语义化方法替代手写失败控制流。

   原因：`require.JSONEq`、`require.Equal`、`require.Contains`、`require.NoError`、`require.ErrorIs` 等方法能直接表达断言意图，并提供更稳定的失败信息。

   备选方案：把 `t.Fatalf` / `t.Errorf` 机械替换为 `require.Failf` 或 `assert.Failf`。该方案保留了低语义失败风格，违反测试说明。

3. 仅在独立字段聚合断言中使用 `assert`。

   原因：HTTP 响应字段有时彼此独立，例如 status、header 和多个 envelope 字段。对这类场景使用 `assert` 可以一次暴露多个字段差异；但初始化、路由注册、请求执行和 JSON 可解析性仍属于前置条件，应使用 `require`。

   备选方案：完全禁止 `assert`。该方案会降低响应字段回归排查效率，不符合用户给定范围。

4. 不引入兼容断言或旧行为双写。

   原因：本变更目标是标准化当前行为测试，不是验证历史行为兼容。旧 header、旧 envelope 或旧 CORS 行为断言会增加误导性覆盖并扩大维护成本。

   备选方案：保留旧行为断言作为兼容保护。该方案会把非目标行为固化为测试契约，与“不做”范围冲突。

5. 规格归属拆分到 `shared-platform-primitives` 和 `runtime-observability`。

   原因：binding、response 和 middleware helper 属于共享平台 primitive；OpenAPI、pprof 和 middleware 可观测性验证与 runtime observability 有交集。两个 delta 均只约束测试治理，不要求生产行为变化。

   备选方案：创建新的测试专用 capability。该方案会把一次性治理任务伪装成长期 capability，不符合能力地图规则。

## Risks / Trade-offs

- [Risk] 机械迁移可能改变测试短路行为，导致部分子断言不再执行。→ Mitigation: 对前置条件使用 `require`，仅对彼此独立字段使用 `assert`，并运行 `go test ./common/http/...`。
- [Risk] JSON 字符串比较迁移不当可能忽略 envelope 结构差异或引入格式敏感失败。→ Mitigation: JSON body 优先使用 `require.JSONEq`；需要结构化字段检查时先 `require.NoError` 解析，再使用语义化字段断言。
- [Risk] 仍保留的 `t.Fatal*`、`t.Error*` 或 `Fail*` 命中可能掩盖未迁移场景。→ Mitigation: 使用指定 `rg` 命令扫描，只有符合 `docs/TESTING.md` 特殊例外规则的命中才能保留，并在 tasks 中列明。
- [Risk] 测试文件新增 `assert` import 后未实际使用或使用场景不当。→ Mitigation: 通过 `go test` 编译检查 import，并在实现中只对独立响应字段聚合使用 `assert`。
- [Risk] OpenSpec delta 被误解为要求生产代码改动。→ Mitigation: delta 明确只约束测试断言规范，tasks 明确禁止生产行为和 user-service transport 测试变更。

## Migration Plan

1. 扫描 `common/http/**/*_test.go` 中 `t.Fatal*`、`t.Error*`、`Fail*` 和现有 `testify` 使用点，建立迁移清单。
2. 按包迁移断言：binding、middleware、openapi、pprof、response。
3. 对 JSON envelope 和 OpenAPI 输出优先使用 `require.JSONEq` 或结构化解析后的 `require.Equal` / `require.Contains`。
4. 对 HTTP status、header、route 存在性、binding 错误、middleware 输出使用语义化 `require` / 必要的 `assert`。
5. 复跑 `rg` 扫描，记录剩余特殊例外及原因。
6. 运行 `go test ./common/http/...` 和 `openspec validate standardize-common-http-test-assertions-no-compat`。

回滚方式：该变更只修改测试和 OpenSpec artifacts；如迁移造成不可接受的测试维护成本，可回滚测试断言迁移和本 change artifacts，不涉及运行时数据、数据库 migration、部署清单或生产回滚。

## Open Questions

- 无。用户已明确变更范围、非目标和验收标准。
