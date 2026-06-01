## 1. 实现

- [x] 1.1 将 `common/middleware/logging.go` 中 `client_ip` 字段改为直接使用 `c.ClientIP()`。
- [x] 1.2 移除 `common/middleware/logging.go` 对 `github.com/aegiscore/common/netutil` 的 import。
- [x] 1.3 删除 `common/netutil/` 目录及其 `ip.go`、`ip_test.go`。
- [x] 1.4 全仓库搜索并确认不存在 `common/netutil`、`netutil.GetClientIP` 或已删除 header 常量的引用。

## 2. 验证

- [x] 2.1 在 `common/` 模块运行 `go test ./...`，确认共享中间件和公共包编译通过。
- [x] 2.2 在 `user-services/` 模块运行 `go test ./...`，确认用户服务引用共享中间件时编译通过。
- [x] 2.3 如格式化检查发现变更，运行 `gofmt -w common/middleware/logging.go`。

## 3. OpenSpec 校验

- [x] 3.1 运行 `openspec status --change "remove-netutil-client-ip"` 确认 artifacts 完整。
- [x] 3.2 确认 delta specs 覆盖 `http-service-runtime` 的 request logging 行为和 `shared-infrastructure` 的共享 IP 提取工具移除。
