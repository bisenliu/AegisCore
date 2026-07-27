## 1. OpenSpec 与设计

- [x] 1.1 创建 `shared-platform-primitives` proposal、design 和 spec delta，明确出站 HTTP helper 的范围、安全默认值和错误语义。
- [x] 1.2 确认默认 Resty client 不承载业务 DTO、认证、重试策略、服务发现、日志或外部系统防腐逻辑。

## 2. 共享 HTTP client

- [x] 2.1 新增 `common/http/client`，实现 `SendRequest`、`NewRequest`、`Send`、`SendContext` 和可检查的 `StatusError`。
- [x] 2.2 在 `common` 引入 `github.com/go-resty/resty/v2`，使用 Resty 编码 query、header、JSON/form body，并支持默认 timeout、context、显式 proxy URL 和调用方 client 注入。
- [x] 2.3 复用无 cookie、无 retry 的默认 Resty client，保持默认 TLS 验证，确保逐请求 timeout 不修改调用方的 maps、`*resty.Client` 或 transport。

## 3. 测试与文档

- [x] 3.1 添加单元测试，覆盖默认值、query/header、JSON/form、method、非 2xx body 与状态错误。
- [x] 3.2 添加单元测试，覆盖 context 取消、timeout、非法输入、默认 TLS 校验、默认无 cookie/retry、显式受信 client、proxy 配置和注入 Resty 重试策略。
- [x] 3.3 更新 `docs/ARCHITECTURE.md`，登记 `common/http/client` 的职责与外部集成边界。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./http/client`（`common` module）。
- [x] 4.2 运行 `openspec validate add-shared-http-request-client`、`openspec list --specs` 和 `openspec validate --specs`。
- [x] 4.3 运行 `make user-service-architecture-lint`。
- [x] 4.4 检查本次 diff，确认只包含预期代码、依赖、测试、文档与 OpenSpec artifacts。
- [x] 4.5 将本次预期变更加到暂存区后运行 `make lint`。
- [x] 4.6 保持本次预期变更处于暂存区后运行 `make verify`。
- [x] 4.7 所有验证通过后，将对应 checkbox 更新为 `- [x]`。
