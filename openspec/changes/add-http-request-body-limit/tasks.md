## 1. 共享 HTTP 能力

- [X] 1.1 在 `common/contract/errors` 中定义请求体超限的稳定应用错误语义、错误码和公开中文消息，并补充错误码治理测试。
- [X] 1.2 在 `common/http/response` 中将请求体超限错误映射为 `413 Payload Too Large`，覆盖直接错误和包装错误的单元测试。
- [X] 1.3 在 `common/http/middleware` 或相邻 HTTP helper 中实现基于 `http.MaxBytesReader` 的业务中立请求体上限 middleware，确保配置非法时 fail-fast。
- [X] 1.4 调整 `common/http/binding` 的 JSON 解码错误归一化，使固定长度、chunked 和尾随 JSON 超限均返回稳定超限错误，而非普通 `400` 或 `500`。
- [X] 1.5 增加共享测试，覆盖合法小 JSON、空 body、未知字段、尾随小 JSON、固定长度超限、chunked 超限和尾随 JSON 超限。

## 2. user-service 配置与装配

- [X] 2.1 在 user-service 服务私有 HTTP 配置中新增请求体最大字节数字段、默认值和严格校验，错误包含完整字段路径；保持业务字段只从配置文件加载。
- [X] 2.2 在 `user-service/internal/providers/transport/gin.go` 的 Gin engine 装配中安装请求体上限 middleware，并保持 tracing、request id、metrics、recovery、日志和 CORS 的可观察顺序。
- [X] 2.3 如需要端点覆盖，在 user-service 边界实现路由或分组策略，避免把 auth/user 路径和服务私有默认值放入 `common`。（当前 auth/user JSON DTO 均适用统一 `64 KiB` 上限，无需端点覆盖。）
- [X] 2.4 增加配置加载测试，覆盖默认值、合法覆盖、零值或负值失败、未知字段仍失败。

## 3. 业务入口测试

- [X] 3.1 为 auth HTTP 入口增加超限测试，覆盖登录、refresh、强制改密的固定长度、chunked 和尾随 JSON 超限，断言返回 `413` 且 use case 未调用。
- [X] 3.2 为 user 创建 HTTP 入口增加超限测试，覆盖固定长度和尾随 JSON 超限，断言返回 `413` 且未创建资料或凭证。
- [X] 3.3 为用户查询或健康探针增加回归测试，确认 GET/query-only 请求不受请求体限制副作用影响。

## 4. 部署与配置资产

- [X] 4.1 更新 user-service 本地配置样例和 Nacos `local-host`、`local-docker` 配置，声明请求体上限默认值。
- [X] 4.2 确认 Compose 发布并选择包含请求体上限的 Nacos 配置，不新增业务字段环境变量覆盖。
- [X] 4.3 确认 Helm chart 只传递 Nacos 来源选择配置，不新增请求体上限 values 或环境变量。
- [X] 4.4 运行部署资产相关结构化测试或 `helm template`/`helm lint` 检查，并确认无 migration Job、OpenAPI 或数据库 drift。

## 5. 验证与收尾

- [X] 5.1 运行相关包测试：`go test ./...` 于 `common`，以及 user-service auth/user/config/transport 相关包测试。
- [X] 5.2 运行 `make user-service-architecture-lint`，确认 common、user-service、deployments 边界未违规。
- [X] 5.3 如 API 注解或生成输入发生变化，运行 `make user-service-openapi-generate` 并检查生成物；如未变化，在实现记录中说明无需生成。（本次未修改 API 注解或 OpenAPI 生成输入，无需生成。）
- [X] 5.4 将本次预期代码、配置、规格和文档变更加到暂存区，再运行 `make lint`。
- [X] 5.5 在暂存预期变更后运行 `make verify`，确认最终 drift 检查不被未暂存预期变更阻塞。
