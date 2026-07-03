## Context

`common/http/response` 是跨服务 HTTP 响应 helper 边界，当前已有统一 `Envelope`、错误码和部分响应 helper 测试。`Created`、`NoContent`、`ValidationFailed`、`Unauthenticated`、`Forbidden`、`Conflict`、`NotFound` 等薄 wrapper 仍缺少直接测试，无法在覆盖率和断言层面明确锁定当前 status、code、message、data 与无 body 行为。

本变更只补齐 `common/http/response` 同包测试和 OpenSpec delta，不改变生产代码、HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或安全边界。

## Goals / Non-Goals

**Goals:**

- 覆盖成功响应 wrapper：`Created` 保持 `201`、`CodeOK`、`created` message 和 `data`；`NoContent` 保持 `204` 且无 body。
- 覆盖错误响应 wrapper：`ValidationFailed`、`Unauthenticated`、`Forbidden`、`Conflict`、`NotFound` 通过统一失败 envelope 输出当前 code、message 和 HTTP status。
- 新增测试遵循 `docs/TESTING.md` 的断言规范，优先使用语义化 `require`/`assert`。
- 通过 `go test -cover ./common/http/response`、`go tool cover -func` 和 `openspec validate cover-response-helpers-no-compat` 验证。

**Non-Goals:**

- 不修改 `common/contract/errors` 错误码、默认消息或 HTTP status 映射。
- 不修改 feature controller、middleware、OpenAPI 注解或调用方式。
- 不新增旧 envelope、旧 `success` 兼容分支、旧 helper alias 或机械断言兼容 helper。

## Decisions

### Decision: 使用同包单元测试直接调用 wrapper

新增测试放在 `common/http/response/response_test.go` 的 `package response` 中，复用现有 Gin test context 和 `contractresponse.Envelope` 解码路径。这样可以直接验证公开 wrapper 的外部响应结果，并保持测试贴近现有包内风格。

备选方案是只测试底层 `WriteError` 或私有 envelope helper。该方案无法证明薄 wrapper 绑定了正确错误构造器和 HTTP status，因此不采用。

### Decision: 显式断言当前 envelope 字段而非兼容旧格式

测试直接断言 `success`、`code`、`message`、`data`、`errors` 和 HTTP status；`NoContent` 断言 body 为空。错误 wrapper 测试不接受旧错误消息、旧错误码或旧 status。

备选方案是只断言 status 或 body 非空。该方案无法防止 envelope 字段漂移，因此不采用。

### Decision: 只补测试，不为测试增加生产分支

当前 wrapper 已经通过 `WriteError`、`JSON` 和 `c.Status` 表达完整行为，缺口是测试覆盖而不是实现能力。变更应保持生产代码最小化，避免为了覆盖率添加冗余接口、分支或适配层。

## Risks / Trade-offs

- [Risk] 表驱动测试过宽会弱化单个 wrapper 的失败定位 -> Mitigation：每个 case 使用清晰名称，并在 case 中声明期望 status、code 和 message。
- [Risk] JSON 解码到 `map[string]any` 会造成数值类型断言噪声 -> Mitigation：成功/错误 envelope 优先解码到 `contractresponse.Envelope`，只在需要检查字段缺失时使用 map。
- [Risk] 覆盖率报告可能受 Go 工具输出格式影响 -> Mitigation：以 `go test -coverprofile` 生成 cover profile，再用 `go tool cover -func` 检查 wrapper 函数。

## Migration Plan

本变更不涉及运行时迁移。回滚时移除新增测试与本 change artifacts 即可恢复原状态；生产二进制行为不变。

验证顺序：

1. 运行 `openspec validate cover-response-helpers-no-compat`。
2. 运行 `go test -cover ./common/http/response`。
3. 运行 `go test -coverprofile=<profile> ./common/http/response` 后用 `go tool cover -func <profile>` 检查目标 wrapper 覆盖。

## Open Questions

无。
