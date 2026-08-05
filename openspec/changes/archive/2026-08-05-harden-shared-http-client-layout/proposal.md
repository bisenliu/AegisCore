## Why

`common/http/client` 的公开类型、错误、默认 client、校验、代理选择和发送流程目前集中在单个 `client.go`，package 只有零散 symbol 注释，没有像 `scheduler` 一样集中说明 context、timeout、client、body 和并发所有权，也缺少可编译的公开 API examples。

发送流程还直接从调用方持有的 `SendRequest` 读取可变 maps，没有形成明确的逐次发送配置快照。需要在保持现有公开 API 和网络语义的前提下整理职责，并明确共享 HTTP primitive 的使用边界。

## What Changes

- 将完整使用契约迁入 `doc.go`，补充可编译、可运行的公开 API examples，并按类型、错误、默认值、构造、校验、client 选择和发送职责拆分同 package 文件。
- `SendContext` 在校验与发送边界创建浅层请求快照，复制 query、form 和 header maps，并裁剪 URL、method 与 proxy URL 的首尾空白。
- 明确 `SendRequest` 可顺序复用但不支持并发修改或并发发送；`JSONData` 中的引用值、`io.Reader` 和注入的 `*resty.Client` 仍由调用方负责生命周期与并发安全。
- 保持默认 60 秒 timeout、安全 TLS、无 cookie、无 retry、form 优先、代理限制、完整 response body 和可检查 `StatusError` 行为。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 明确共享 HTTP client 的逐次发送配置快照、浅拷贝范围和调用方所有权约束。

## Impact

- Go 代码：`common/http/client/` 及其测试。
- 共享契约：不新增、删除或重命名导出符号；URL、method 和 proxy URL 的首尾空白改为在发送前裁剪。
- 调用方：仓库内没有生产调用方，无需迁移；合法请求、注入 client、proxy、timeout 和错误处理方式保持不变。
- 依赖：继续使用当前 `github.com/go-resty/resty/v2`，不新增依赖。
- 不影响 HTTP API、数据库 schema/migration、Ent/OpenAPI 生成物、部署清单、观测指标、日志字段或安全边界。
