## 1. 共享工具迁移

- [x] 1.1 在 `common/netutil` 新增客户端 IP 提取工具包，迁移 `GetClientIP` 和代理头常量。
- [x] 1.2 优化 `X-Forwarded-For` 解析，跳过空白候选值并统一 trim 返回值。
- [x] 1.3 保持 fallback 顺序为 `X-Forwarded-For`、`X-Real-IP`、`X-Client-IP`、Gin `ClientIP()`。

## 2. 共享中间件集成

- [x] 2.1 更新 `common/middleware.RequestLogger`，使用 `common/netutil.GetClientIP` 填充 `client_ip` 日志字段。
- [x] 2.2 确认日志字段名、trace-id 行为和 middleware 调用顺序不变。

## 3. 测试与验证

- [x] 3.1 为 `common/netutil.GetClientIP` 添加单元测试，覆盖多 IP、空白候选、`X-Real-IP`、`X-Client-IP` 和 Gin fallback。
- [x] 3.2 为请求日志复用共享 IP 提取逻辑补充或调整测试，验证代理头值进入 `client_ip` 字段。
- [x] 3.3 在 `common/` 运行 `gofmt` 和 `go test ./...`，确保共享模块通过验证。
