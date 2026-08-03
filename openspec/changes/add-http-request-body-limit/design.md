## Context

`common/http/binding.JSONBinder` 当前直接从 `c.Request.Body` 解码 JSON，并通过第二次 `Decode(&extra)` 检查尾随 JSON 文档。该实现能发现拼接 JSON，但没有在解码前建立硬字节边界；当请求体为超大固定长度、chunked 传输，或首个合法对象后跟随巨型 JSON 时，解码器仍可能在 binder、校验和业务逻辑前消耗大量堆内存与 CPU。

user-service 的公开登录、refresh、强制改密和用户创建接口均通过共享 binder 或相同 HTTP 入口处理请求。当前 Helm 默认内存 limit 为 `512Mi`，缺少服务内请求体上限会让网关配置漂移、内部直连或恶意客户端绕过外层限制后直接打穿 Pod 资源预算。

本变更跨 `common`、`user-service` 和 `deployments`：`common` 提供业务中立 primitive 和错误映射，user-service 拥有服务私有默认值、端点覆盖和 Gin 装配，Nacos 环境资产声明与运行时一致的默认配置。

## Goals / Non-Goals

**Goals:**

- 在 JSON 解码前对入站请求体建立硬字节上限，覆盖固定长度、chunked 和尾随 JSON 载荷。
- 对超限请求返回稳定 `413 Payload Too Large` 和统一 response envelope，并保证不进入 feature controller 后续 use case。
- 提供 user-service 默认上限和少量端点覆盖能力，使 auth 与 user 写接口无需各自手写限制逻辑。
- 同步本地 Nacos 环境配置默认值，确保运行时容量预算与部署资源边界一致；Compose 和 Helm 继续只选择 Nacos 配置来源。
- 增加结构化测试，覆盖 shared binder/middleware、auth 公开入口和用户写接口的超限行为。

**Non-Goals:**

- 不引入数据库 schema、Ent 生成物或 Atlas migration。
- 不改变 HTTP 路径、认证 token、RBAC 授权、OpenAPI endpoint 列表或业务 DTO 语义。
- 不实现全局速率限制、WAF、streaming upload、multipart 文件上传或按用户配额。
- 不把 user-service 私有端点上限、路径表或业务策略放入 `common`。

## Decisions

- 在 Gin 入口安装业务中立 body limit middleware，而不是在每个 controller 中调用 `http.MaxBytesReader`。
  这样可以在 binder 和 use case 前统一拒绝超限请求，避免 100+ API 依赖人工记忆。备选方案是在 `binding.JSONBinder` 内包裹 body，但 binder 无法可靠访问 `ResponseWriter` 完成 `MaxBytesReader` 的连接处理语义，也不适合表达端点覆盖策略。

- `common` 只提供无业务语义的限制 primitive 和错误映射，user-service 负责配置、默认值和路由策略。
  这样保持 `common` 跨服务可复用，不引入 auth/user 路径、业务 DTO 或服务私有配置。备选方案是把完整配置 schema 放到 `common/runtime/config`，但请求体预算属于消费服务的 HTTP 暴露策略，应留在服务配置边界。

- 请求体上限只使用 user-service 配置文件，不新增专用环境变量覆盖。
  当前配置体系由环境变量定位 Nacos，业务字段来自分层 YAML；保持单一配置来源可避免环境变量与 Nacos 值产生优先级和漂移。Compose 负责发布并选择本地 Nacos 配置，Helm 负责选择目标 Nacos namespace/group/dataId。

- 超限错误统一映射为应用错误再由 `common/http/response` 渲染 `413`。
  这样保持 response envelope、错误码和公开文案稳定，并使测试不依赖 Go 标准库错误文本。备选方案是 middleware 直接写裸 `413`，但会破坏现有 response helper 契约和客户端错误处理一致性。

- 默认上限选择保守的小 JSON 预算，并允许 auth 或用户写接口按需覆盖。
  登录、refresh、强制改密和创建用户请求体都应远小于通用上限；保守默认能降低 OOM 风险。备选方案是只依赖 Kubernetes ingress 或网关 body size 限制，但这无法覆盖内部直连、配置漂移和本地部署。

- 尾随 JSON 检查继续保留，但必须在已受限 body reader 上执行。
  这样既保留当前防止拼接载荷隐藏尾随数据的语义，又确保尾随巨型 JSON 不会绕过容量预算。

## Risks / Trade-offs

- [Risk] 默认上限过小导致未来合法大请求被拒绝。→ 通过服务私有配置和端点覆盖处理真实大 JSON 需求，新增大 body API 必须在设计中声明容量预算。
- [Risk] middleware 只限制带 body 的请求，GET 或健康探针不应受影响。→ 限制逻辑按 method/content length/body presence 判断，测试覆盖 runtime endpoint 与普通 JSON 写请求。
- [Risk] `http.MaxBytesReader` 需要正确使用原始 `ResponseWriter`。→ middleware 必须在 Gin handler 链最前侧包裹 `c.Request.Body`，并保持后续 middleware、metrics、日志和 recovery 可观察到 `413`。
- [Risk] 错误映射不完整会把超限错误渲染为 `400` 或 `500`。→ 在 `common/contract/errors` 和 `common/http/response` 增加专门测试，验证 `*http.MaxBytesError` 或包装后的应用错误映射为 `413`。
- [Risk] Nacos 配置与代码默认值漂移。→ 更新 `local-host`、`local-docker` 的服务配置，并用结构化配置测试约束关键字段。

## Migration Plan

- 先实现 `common` 的 body limit primitive、超限错误和 `413` 映射，保证共享测试通过。
- 再在 user-service 配置中加入默认值和校验，并在 Gin engine 装配中安装 middleware。
- 然后更新本地 Nacos 配置；Compose 和 Helm 沿用既有 Nacos 配置来源选择方式，使新版本启动时默认启用请求体上限。
- 发布时无需数据库迁移；普通滚动发布即可生效。若上线后发现合法请求被误拒，可通过配置提高对应端点或默认上限并滚动重启。

## Open Questions

- 默认字节上限的具体数值在实现阶段根据现有 DTO、配置风格和测试 fixture 确认；初始建议使用足以覆盖当前 JSON DTO 的保守值，并在配置注释中说明单位。
