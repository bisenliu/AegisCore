## Context

`common/netutil` 当前只包含 Gin 客户端 IP 提取工具，并且唯一业务引用位于 `common/middleware/logging.go`。现有主规格 `shared-infrastructure` 将该工具定义为共享能力，`http-service-runtime` 要求 request logging 输出 `client_ip` 字段。

Gin 已提供 `Context.ClientIP()` 作为框架级客户端 IP 解析入口，解析行为可随 Gin engine 的代理配置统一调整。继续在 `common` 中维护自定义 header 优先级会让 request logging 与 Gin runtime 的受信代理策略脱节。

## Goals / Non-Goals

**Goals:**

- 删除 `common/netutil` 包及其测试。
- 将共享 request logging middleware 中 `client_ip` 字段的取值改为 `c.ClientIP()`。
- 保持 HTTP API、响应信封、路由注册、日志字段名和 trace-id 行为不变。
- 更新 OpenSpec delta，反映共享 IP 提取工具被移除，request logging 改用 Gin 客户端 IP 解析。

**Non-Goals:**

- 不新增客户端 IP 解析配置项。
- 不调整 Gin 受信代理或 forwarded header 策略。
- 不改变 controller/service/repository 分层、数据库模型、Ent 代码或 Atlas migration。
- 不改变 HTTP 响应契约、错误码或业务 API 行为。

## Decisions

- 使用 Gin `Context.ClientIP()` 替代 `netutil.GetClientIP(c)`。
  - 理由：Gin 是当前 HTTP runtime 的事实入口，使用框架 API 可以与 Gin engine 的代理配置保持一致。
  - 备选方案：保留 `common/netutil` 但内部直接调用 `c.ClientIP()`。该方案仍保留无额外价值的包装包，无法满足删除目录的目标。

- 删除 `common/netutil` 测试而不是迁移等价测试。
  - 理由：删除自定义解析后，`X-Forwarded-For`、`X-Real-IP`、`X-Client-IP` 的具体解析细节不再由本仓库维护，测试 Gin 内部行为会增加脆弱性。
  - 备选方案：为 request logging 增加 Gin `ClientIP()` 字段测试。当前仓库没有针对 request logging 输出字段的稳定测试辅助，新增测试不是完成该重构的必要条件。

- 将规格变更拆分到 `shared-infrastructure` 和 `http-service-runtime`。
  - 理由：`shared-infrastructure` 删除共享工具能力，`http-service-runtime` 保持请求日志 `client_ip` 字段但调整来源。

## Risks / Trade-offs

- [Risk] 日志中的 `client_ip` 值可能与旧自定义解析在代理 header 存在时不同。→ Mitigation: 明确该字段遵循 Gin `ClientIP()` 规则，并保持字段名不变以降低日志消费端改造成本。
- [Risk] 仓库外部若直接 import `common/netutil` 会编译失败。→ Mitigation: proposal 标记为 breaking，迁移方式为调用方改用 Gin `Context.ClientIP()`。
- [Risk] Gin 代理配置不当会影响 `ClientIP()` 结果。→ Mitigation: 本变更不引入新的代理策略，后续如需配置化受信代理应作为独立 runtime 变更提出。
