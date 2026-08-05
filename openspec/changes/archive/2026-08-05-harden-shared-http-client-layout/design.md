## Context

`common/http/client` 是业务中立的轻量出站 HTTP primitive，当前用 `SendRequest` 集中表达 URL、method、query、JSON/form、header、proxy、timeout 和可注入 Resty client。实现安全地复用默认 client，并通过 request context 提供逐请求 timeout，但全部职责位于一个文件，所有权和复用边界只能从实现与历史 change 中推断。

本次只影响 `common/http/client/` 与 `shared-platform-primitives` 规格。真实外部系统 DTO、认证、retry policy、业务错误映射和防腐逻辑继续属于 `user-service/internal/integration/http` 或消费 feature；不得移入 common。`user-service` 当前没有生产调用方，`deployments`、HTTP API、OpenAPI、数据库和观测资产均不需要同步修改。

## Goals / Non-Goals

**Goals:**

- 对齐 `scheduler` 的 package 文档、executable examples、单一职责文件和配置所有权表达方式。
- 在每次发送开始时固化 request-level 标量和 map 配置，避免 Resty 请求构造继续引用调用方 maps。
- 明确 shallow snapshot、body/client 所有权、顺序复用和并发限制。
- 保持全部现有导出符号、默认值、安全设置、编码优先级和错误语义。

**Non-Goals:**

- 不设计新的 fluent builder、泛型 response、retry、认证、tracing、logging、service discovery 或业务 DTO。
- 不 deep-copy `JSONData`，不缓存或重放 `io.Reader`，不 clone 调用方注入的 Resty client。
- 不声称 `SendRequest` 支持并发修改或并发发送。
- 不替换 Resty，不改变默认 TLS、cookie、retry、proxy 或 response body limit 策略。

## Decisions

### Decision: 在同一 package 内按职责拆分

package 契约放入 `doc.go`，公开示例放入外部测试 package 的 `example_test.go`；公开类型、错误、默认值、构造/入口、校验/快照、client 选择和实际发送分别由小文件承载。所有导出符号继续位于 `github.com/aegiscore/common/http/client`，不创建子包或兼容 wrapper。

备选方案是只扩写 `client.go` 注释。该方案仍把导航、所有权说明和实现细节混在一起，也不能提供由 `go test` 校验的调用示例，因此不采用。

### Decision: 每次发送形成浅层配置快照

`SendContext` 先检查 nil receiver 与 nil context，再复制 `SendRequest` 的标量字段，并使用标准库 map clone 复制 `QueryParams`、`FormData` 和 `Headers`。随后在快照上裁剪 URL、method、proxy URL、填充零值 timeout、完成校验，并仅把快照交给 Resty 请求构造与 client 选择。

快照是浅拷贝：`JSONData` 可能包含 map、slice、pointer 或 `io.Reader`，`RestyClient` 也是调用方拥有的指针。通用 helper 无法在不改变类型语义、流式行为或自定义编码的情况下安全 deep-copy 它们，因此调用方必须在 `SendContext` 返回前保持这些对象稳定。一个 `SendRequest` 可以在前一次发送返回后修改并再次发送，但不能并发修改或并发发送。

备选方案是为 `SendRequest` 增加 mutex。公开字段可绕过 setter，mutex 无法保护外部 map、body 或 Resty client 的修改，还会制造虚假的并发安全承诺，因此不采用。另一个方案是改成不可变 builder；这会造成不必要的 breaking API，留给独立 change 评估。

### Decision: 只归一化首尾空白，不扩大 URL/method 校验

URL、method 和 proxy URL 使用裁剪后的字符串进行校验与发送，使非空校验和实际执行一致。URL 继续允许注入 Resty client 的 BaseURL 配合相对路径，method 继续交给 Go/Resty 校验；本次不强制绝对 HTTP(S) request URL，也不自动大写 method。ProxyURL 继续要求绝对 HTTP(S) URL且不能与注入 client 并用。

备选方案是强制 request URL 为绝对 HTTP(S) URL。该方案会破坏 Resty BaseURL 的合法组合，超出布局和所有权优化范围，因此不采用。

### Decision: 示例覆盖稳定使用契约

executable examples 覆盖基本 JSON 请求、调用方 context 取消、非 2xx `StatusError` 和注入 client。示例只使用本地 `httptest` 或固定 transport，不访问公网，不引入生产测试 hook。

## Risks / Trade-offs

- [Risk] 调用方误以为 shallow snapshot 能保护并发修改 `JSONData` 或注入 client。 -> Mitigation：package 文档、字段注释和 examples 明确 body/client 的调用方所有权及并发限制。
- [Risk] 裁剪 URL 或 method 空白改变此前最终由 Resty 报错的非法输入结果。 -> Mitigation：这是发送前归一化；合法无空白输入和所有导出 API 保持不变，并增加聚焦测试。
- [Risk] 文件拆分过细增加跳转。 -> Mitigation：只按公开契约、构造、校验、transport 和发送等稳定职责拆分，不引入抽象层或子包。
- [Trade-off] shallow snapshot 不复制任意 body graph。 -> Mitigation：通用 deep-copy 不可靠；明确调用方必须维持 body 稳定，流式 reader 默认只适合单次发送。

## Migration Plan

1. 新增 `shared-platform-primitives` spec delta，锁定浅层快照与所有权语义。
2. 在同一 package 内重组实现并补充 package 文档，不改变导出 API。
3. 增加 examples、快照/裁剪测试，运行普通测试、race 和 vet。
4. 运行 OpenSpec validation、架构 lint、`make lint` 和 `make verify`。

回滚时整体回退本 change 的 `common/http/client`、规格和文档文件即可；没有数据库、生成物、部署或生产调用方迁移。

## Open Questions

无。不可变 builder、泛型 response、retry、认证和观测接入必须作为独立需求评估，不能借本次布局优化进入 common。
