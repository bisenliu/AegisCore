# Design

## Overview

本变更分两层推进：

```text
主规则更新
  -> 在 AGENTS.md 与 docs/ARCHITECTURE.md 固化注释语言和日志语言约束

代码统一执行
  -> 审计日志覆盖与等级
  -> 翻译非生成源码注释
  -> 更新测试与验证脚本
```

实现时先建立可复用扫描清单，再按模块分批处理，避免盲目替换导致生成代码、字符串常量、错误消息或文档历史记录被误改。

## Source Scope

本次审计覆盖：

- `common/` 非生成 Go 源码。
- `user-service/` 非生成 Go 源码。
- 必要的测试文件。
- 长期主规则文档：`AGENTS.md`、`docs/ARCHITECTURE.md`。

明确排除：

- `user-service/ent/` 生成代码，但 `user-service/ent/schema/` 可按人工维护源码处理。
- Swagger generated docs、Atlas migration、`go.sum`、工具缓存、第三方依赖和历史 change 记录。
- 不属于日志消息的 error string、HTTP response message、配置 key、字段名、常量名和 Go identifier。

## Main Rule Updates

主规则新增以下约束：

- 代码注释统一使用中文；函数和方法注释必须使用中文。
- 日志消息内容必须使用英文；日志字段名保持英文 snake_case。
- 日志等级必须表达场景严重性，不能用 `Error` 表达预期业务拒绝，也不能用 `Info` 掩盖系统异常。
- 业务日志优先使用 `common/runtime/logger` context helper，保证 trace-id 自动进入日志字段。

`AGENTS.md` 保持短规则入口，放在 Repository Rules 中。`docs/ARCHITECTURE.md` 在 Logging And Trace ID 一节记录完整语义。

## Logging Audit Model

日志审计按调用点分类，而不是只按文本匹配。

### Runtime And Resource Lifecycle

适用范围：

- HTTP server start/stop。
- PostgreSQL、Redis、Ent client lifecycle。
- worker pool stop、task failure、panic recover。
- logger 初始化和 Fx lifecycle。

建议：

- 成功启动、连接、停止使用 `Info`。
- 生命周期取消、server closed 等常规细节使用 `Debug`。
- 关闭失败、监听失败、资源 ping 失败、worker task error 使用 `Error`。

### HTTP Middleware

适用范围：

- request logging。
- auth middleware。
- validation binding。
- panic recovery。
- trace-id 传播。

建议：

- HTTP request completed：`2xx/3xx` 使用 `Info`，`4xx` 使用 `Warn`，`5xx` 使用 `Error`。
- 缺少认证 header 可按产品策略使用 `Info` 或 `Warn`；若属于常见未登录访问，倾向 `Info`，避免噪声。
- token 格式错误、token version mismatch、认证拒绝使用 `Warn`。
- token 验证内部错误、validator 依赖失败、panic recover 使用 `Error`。

### Application Use Cases

适用范围：

- 用户创建、查询、列表。
- 登录、刷新、强制改密、退出当前设备、退出全部设备。
- session/token version validation 和 fallback。

建议：

- 重要业务动作开始或成功完成使用 `Info`，避免在高频读路径写过多冗余日志。
- 用户不存在、密码不匹配、状态拒绝、token mismatch、refresh session mismatch 使用 `Warn`。
- 密码 hash/JWT 签发/数据库访问/Redis 访问/缓存回填失败使用 `Error`。
- 如果同一个 error 会在更高层被完整记录，低层只补充必要上下文字段，避免重复 noisy logging。

### Infrastructure Adapters

适用范围：

- PostgreSQL adapter。
- Redis session/token version adapter。
- future integration adapter。

建议：

- 一般存储 adapter 返回 error 即可，除非该层有异步任务、重试、fallback、批量处理或无法被调用方感知的失败。
- 后台任务执行失败必须在执行处记录 `Error`，因为请求链路已经无法返回该错误。
- 缓存失败后回退数据库可使用 `Warn`，数据库 fallback 失败使用 `Error`。

## Missing Log Heuristics

补日志时优先处理以下场景：

- 错误被吞掉、异步执行或无法返回给调用方。
- 外部依赖失败且调用方难以补齐资源上下文。
- 认证、安全、会话撤销和 token version 相关拒绝。
- 可能影响数据一致性、缓存一致性或后台清理的路径。
- 服务生命周期和资源生命周期关键节点。

避免补日志的场景：

- 纯函数、值对象校验、简单 DTO 映射。
- 会被 HTTP request logger 和上层 application logger 完整覆盖的普通返回路径。
- 高频循环内部没有聚合价值的逐条日志。
- 测试专用 helper 中不会进入运行时观测面的信息。

## Comment Translation Model

注释翻译只处理人工维护源码注释：

- `// Package ...`
- `// TypeName ...`
- `// FuncName ...`
- 复杂逻辑前的解释性注释。
- 测试中解释 mock、fixture、断言意图的注释。

翻译原则：

- 保留 Go doc 注释格式，导出标识符注释仍以对应 identifier 开头，后接中文说明。
- 保留代码术语、协议名、HTTP、JWT、Redis、PostgreSQL、Ent、Fx、Gin、trace-id 等必要英文术语。
- 不为了翻译注释而改变代码行为、identifier、错误字符串或日志消息。
- 删除无价值注释优于机械翻译，例如“sets field”这类重复代码本身的注释。

示例：

```go
// NewSessionStore 构造认证 Redis session store。
func NewSessionStore(...)
```

## Tooling Approach

使用扫描命令生成候选清单：

```bash
rg -n "logger\\.(Debug|Info|Warn|Error)|\\.Debug\\(|\\.Info\\(|\\.Warn\\(|\\.Error\\(" common user-service --glob '*.go'
rg -n "^\\s*//\\s*[A-Za-z]" common user-service --glob '*.go' -g '!user-service/ent/**'
rg -n "\"[^\"]*[\\p{Han}][^\"]*\"" common user-service --glob '*.go'
```

扫描结果必须人工判断：

- 中文字符串不一定是日志，可能是测试数据或业务 message。
- 英文注释不一定需要翻译，如果位于生成代码或外部样例中应排除。
- 英文日志消息是目标状态，不应翻译。

## Testing Strategy

变更不应改变业务语义，但可能触碰大量源码注释和日志调用，因此验证要覆盖：

- `gofmt` 所有修改过的 Go 文件。
- `make test-common`
- `make test-user-service`
- 日志消息断言相关单测。
- 结构扫描确认非生成源码无英文注释残留。
- 结构扫描确认日志消息无中文内容。

如果完整测试因本地 PostgreSQL/Redis 等外部依赖不可用而失败，需要在结果中明确说明失败原因，并至少运行可独立执行的单包测试。

## Risk And Mitigation

风险：

- 机械替换误改生成代码、error string、HTTP message 或测试断言。
- 日志补充过多导致高频路径噪声增加。
- 日志级别调整影响现有测试或观测告警。
- 导出函数注释改中文时破坏 Go doc 格式。

缓解：

- 分模块小批量修改，每批运行针对性测试。
- 优先修改人工维护文件，显式排除生成代码。
- 对高频路径保持克制，只记录关键状态和异常。
- 通过测试与扫描命令验证语言规则。
