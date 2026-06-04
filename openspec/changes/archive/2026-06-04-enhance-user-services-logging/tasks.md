## 1. 日志字段与辅助约定

- [x] 1.1 梳理 `common/logger` 现有 context API、`trace-id` 字段和堆栈 helper，确认无需新增 logger 初始化或 writer。
- [x] 1.2 确认 `user-services` 继续直接调用 `common/logger`，不新增私有日志框架或 module/method/event/error_code/error_kind/reason 字段 helper。
- [x] 1.3 明确敏感字段禁止清单，确保实现不记录密码、新密码、password hash、access token、refresh token、password-change token、Authorization header、Redis session payload、DSN 或数据库密码。

## 2. 用户资料流程日志

- [x] 2.1 优化 `user_service.CreateUser` 日志，补充 username、status、冲突分支和系统错误堆栈。
- [x] 2.2 优化 `user_service.GetUserByID` 日志，补充 user_id、not found 业务分支和系统错误堆栈。
- [x] 2.3 优化 `user_service.ListUsers` 日志，仅保留 page、page_size 和系统错误堆栈，避免打印过细筛选条件。
- [x] 2.4 确认用户资料日志增强不改变 `common/response.Envelope`、错误码、HTTP 状态码或 DTO 输出。

## 3. HTTP 请求日志中间件

- [x] 3.1 评估 `common/middleware/logging.go` 中 `RequestLogger` 当前全部使用 info 的行为，并确定 2xx/3xx=info、4xx=warn、5xx=error 的级别映射。
- [x] 3.2 修改 `RequestLogger`，按响应状态码选择日志级别，同时保留 method、path、status、latency、client_ip 和 `trace-id` 字段。
- [x] 3.3 修改 `RequestLogger`，从认证 context 获取 `user_id` 并写入日志字段；无法获取时记录 `user_id=anonymous`。
- [x] 3.4 确认 `RequestLogger` 不记录 Authorization header、token、请求 body 或其他敏感字段。

## 4. 认证与会话流程日志

- [x] 4.1 优化登录认证日志，区分用户不存在、密码不匹配、状态不允许、密码校验系统错误和登录成功分支，避免记录密码或 hash。
- [x] 4.2 优化强制改密日志，覆盖改密 token 校验、用户查询、状态不允许、密码哈希、凭证更新和 token version 缓存失效异常。
- [x] 4.3 优化 refresh token 日志，覆盖 token 解析失败、session miss、token version mismatch、Redis/仓储异常和 token 轮转失败，避免记录 token 原文。
- [x] 4.4 优化 logout 和 logout all 日志，覆盖认证上下文缺失、session 删除、token version 增加、缓存失效和用户会话删除失败。
- [x] 4.5 优化 token 签发与认证会话创建错误日志，补充 user_id、session_id、token_version 和堆栈。

## 5. Redis 会话仓储日志

- [ ] 5.1 为 token version cache 读取、解析异常、回源查询、写入和失效失败补充可检索日志字段。
- [ ] 5.2 为认证 session 创建、读取、删除、删除全部用户会话和过期清理失败补充 Redis 操作类型、user_id 或 session_id 和底层错误。
- [x] 5.3 确认 repository 层日志不记录 Redis session JSON payload、token、完整 key 中的敏感值或连接凭证。
- [x] 5.4 避免 service 与 repository 对同一可预期业务 miss 重复记录 error 级日志。

## 6. 测试与验证

- [x] 6.1 补充或调整 `common/logger` 测试，验证 context logger 保持 `trace-id`、共享编码字段和堆栈字段行为。
- [x] 6.2 补充 `common/middleware` 的 `RequestLogger` 测试，验证 2xx/3xx 输出 info、4xx 输出 warn、5xx 输出 error，并验证 `user_id` 存在或为 `anonymous`。
- [ ] 6.3 补充 `user-services/internal/service` 日志相关单元测试或捕获型 logger 测试，验证代表性 error/warn 日志包含必要业务字段且不包含敏感字段。
- [ ] 6.4 补充 `user-services/internal/repository/redis` 测试，验证 Redis 系统错误日志包含操作上下文且不包含 session payload 或 token。
- [x] 6.5 回归 controller/service 现有响应测试，确认日志增强不改变 API 响应信封、错误码和状态码。
- [x] 6.6 在 `common/` 运行 `go test ./...`。
- [x] 6.7 在 `user-services/` 运行 `go test ./...`。

## 7. 格式化与文档同步

- [x] 7.1 对修改的 Go 文件运行 `gofmt -w`。
- [x] 7.2 如新增长期日志字段 helper 或约定，更新相关开发文档或能力说明。
- [x] 7.3 确认未修改 Ent 生成代码、Atlas migration、Redis key 格式、HTTP 路由或配置 schema。
