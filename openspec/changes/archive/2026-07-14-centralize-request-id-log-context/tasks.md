## 1. Logger 关联上下文

- [x] 1.1 在 `common/runtime/logger` 定义 `RequestIDField`、`WithRequestID` 和 `RequestIDFromContext`，使用 logger 私有 context key 保存非空 request ID，并使 `fieldsFromContext` 独立提取有效 trace/span 与 request ID。
- [x] 1.2 扩展 `common/runtime/logger` 单元测试，覆盖仅 request ID、仅有效 span、三字段并存、无有效关联上下文以及 `FromContext`/`WithContext` 自动附加字段的行为。

## 2. HTTP middleware 不兼容迁移

- [x] 2.1 修改 `common/http/middleware.RequestID` 使用 logger request ID context API，删除 middleware 包中的 `RequestIDField`、私有 request ID context key、`WithRequestID` 和 `RequestIDFromContext`，不得保留别名、转发函数或 deprecated wrapper。
- [x] 2.2 删除 `requestLogFields` 对 request ID 的手工读取和追加，更新 HTTP middleware 测试，验证 `X-Request-ID` 透传、生成、非法值替换、trace/span 并存和 access log 中 `request_id` 字段唯一。
- [x] 2.3 将 common 与 `user-service/internal/providers/gin_request_id_test.go` 的全部旧 middleware API 引用原子迁移到 logger API，并使用 `rg` 确认仓库中不存在旧符号定义、引用或兼容入口。

## 3. 应用日志行为验证

- [x] 3.1 扩展 `common/http/binding` 测试，在请求 context 写入 logger request ID，验证 `BindOrAbort` 的 `invalid request` 日志自动包含相同 `request_id`，且 binding 生产代码不手工读取或追加该字段。
- [x] 3.2 运行定向测试：`go test ./common/runtime/logger ./common/http/middleware ./common/http/binding ./user-service/internal/providers`，修复 request ID context、HTTP 接线和应用日志关联回归。

## 4. 规格与架构验证

- [x] 4.1 运行 `openspec validate centralize-request-id-log-context`，确认 proposal、design、`runtime-observability` delta spec 和 tasks 一致且可解析。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认 logger、HTTP middleware、binding 与 user-service provider 的依赖方向符合 common 和服务边界。
- [x] 4.3 检查本 change 未修改 HTTP endpoint、响应 envelope、OpenAPI 生成物、Ent schema、Atlas migration、metrics label、部署清单或观测资产；若任何生成或检查命令产生 drift，移除非本 change 必需的变更并记录原因。

## 5. 完整验证与交付

- [x] 5.1 运行 `make test`，修复本 change 引入的跨模块测试失败。
- [x] 5.2 将本次预期代码、测试、规格和 change artifacts 加到暂存区，并检查 staged diff 只包含 `centralize-request-id-log-context` 所需变更。
- [x] 5.3 运行 `make lint`，修复全部 lint 失败；未通过前不得标记本任务完成。
- [x] 5.4 运行 `make verify`，确认完整验证及最终生成物 drift 检查通过；未通过前不得标记本任务完成。
- [x] 5.5 在每项实现或验证真实完成后立即更新本 `tasks.md` checkbox，并在最终交付前重新暂存 checkbox 变更，确认 change 的所有任务均为 `- [x]`。
