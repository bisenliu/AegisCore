## Context

`common/http/middleware/cors.go` 暴露 `CORS()` 与 `CORSWithOptions(CORSOptions)` 两个入口。当前 `CORSWithOptions` 已有一条自定义策略测试，但默认入口 `CORS()` 没有直接覆盖，`go test -cover` 会显示该 wrapper 仍为 0%。本次 change 只补齐 CORS 默认策略和同域自定义策略稳定字段的测试，不改变生产代码、服务路由挂载、HTTP API、OpenAPI、数据库、部署或观测资产。

## Goals / Non-Goals

**Goals:**

- 直接覆盖 `common/http/middleware.CORS()`，确认它等价于 `CORSWithOptions(defaultCORSOptions)`。
- 锁定当前默认策略：`Access-Control-Allow-Origin=*`、默认 method/header 列表、`OPTIONS` 预检 `204` 短路和普通请求继续进入业务 handler。
- 补齐 `CORSWithOptions` 当前稳定字段的测试断言，覆盖 allow methods、allow headers 和 exposed headers 等已有配置行为。
- 新增或修改测试遵循 `docs/TESTING.md` 的断言风格，常见断言使用 `require`，不引入机械 `Fail`/`Failf` 替换。

**Non-Goals:**

- 不修改 `cors.go` 的生产行为或新增 CORS 配置项。
- 不修改 user-service 运行时是否挂载 CORS middleware 的策略。
- 不修改 auth、request ID、logging、metrics、tracing、recovery 或 Casbin middleware。
- 不新增旧 CORS header、旧 wildcard+credentials 兼容行为、旧 origin 反射默认值或兼容开关。

## Decisions

### Decision: 用真实 Gin engine 覆盖 wrapper 行为

测试通过 `gin.New()` 注册 middleware 和最小 handler，分别执行普通请求与 `OPTIONS` 预检请求。这样可以同时验证 response header、HTTP status、handler 是否被调用，以及 `AbortWithStatus` 对预检的短路效果。备选方案是直接构造 `gin.Context` 调 middleware 函数，但那会绕开 Gin middleware 链和路由行为，无法完整表达普通请求继续进入业务 handler 的稳定语义。

### Decision: 默认策略使用字面期望值锁定，同时比较 wrapper 等价性

默认入口测试同时断言当前字面默认值，并将 `CORS()` 的输出与 `CORSWithOptions(defaultCORSOptions)` 的相关响应结果比较。这样既能证明 wrapper 使用默认选项，也能在默认值意外漂移时通过字面断言暴露。备选方案是只引用 `defaultCORSOptions` 生成期望值，但那会让默认值变更自动通过测试，无法满足本次固化当前默认策略的目标。

### Decision: 保持测试在 `common/http/middleware` package 内

测试保持在同包内，允许比较 `defaultCORSOptions`，不新增只为测试服务的导出 API、adapter 或生产分支。备选方案是通过外部测试包验证导出行为，但无法直接确认 `CORS()` 与 `CORSWithOptions(defaultCORSOptions)` 的等价关系。

## Risks / Trade-offs

- [Risk] 默认策略测试引用 `defaultCORSOptions` 需要同包测试，可能减少外部调用者视角。-> Mitigation：同一测试仍通过导出的 `CORS()` 和 `CORSWithOptions` 入口走完整 Gin 请求链，并用字面响应头验证外部可见行为。
- [Risk] CORS 测试如果散落在 logging 测试文件中会增加维护成本。-> Mitigation：将新增覆盖放在 `cors_test.go`，只在必要时收敛既有 CORS 测试断言。
- [Risk] 整仓库验证可能被其他未提交 change 或 Multica runtime 文件影响。-> Mitigation：优先运行本 change 相关验证；若运行封装验证，报告非本次变更导致的阻塞，不把 unrelated dirty worktree 作为本 change 失败依据。

## Migration Plan

本次只修改测试和 OpenSpec change artifacts，无数据库迁移、OpenAPI 生成、部署发布或运行时配置变更。回滚时移除本 change 新增/修改的测试和 `openspec/changes/cover-cors-default-entry-no-compat/` 即可恢复原状态。

## Verification

- 运行 `openspec validate cover-cors-default-entry-no-compat`。
- 运行 `go test -cover ./common/http/middleware`。
- 运行 `go test -coverprofile=<profile> ./common/http/middleware` 后用 `go tool cover -func <profile>` 确认 `CORS` 有覆盖，且 `CORSWithOptions`、`normalizeCORSOptions` 覆盖率不降低。
- 检查新增测试未使用不符合 `docs/TESTING.md` 规范的 `t.Fatal`、`t.Error`、`Fail` 或 `Failf`。

## Open Questions

无。
