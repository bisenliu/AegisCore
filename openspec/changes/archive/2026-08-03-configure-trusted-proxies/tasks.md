## 1. 配置契约

- [x] 1.1 在 `common/runtime/config.HTTPServerConfig` 新增 `TrustedProxies []string`，mapstructure 键为 `trusted_proxies`，默认值保持空列表或 nil。
- [x] 1.2 更新配置加载、严格解码、effective render 和 loader 测试，验证 `server.http.trusted_proxies` 可读取且 `http.trusted_proxies` 继续被拒绝。
- [x] 1.3 如配置校验已有 HTTP server 校验入口，补充 trusted proxy IP/CIDR 非法值测试，并确保错误定位到 `server.http.trusted_proxies`。

## 2. Gin Engine 行为

- [x] 2.1 更新 `user-service/internal/providers/gin.go`，使用 `params.Config.Server.HTTP.TrustedProxies` 调用 `engine.SetTrustedProxies`，移除“应用不接受 trusted proxy 配置”的实现注释。
- [x] 2.2 更新 `user-service/internal/providers/gin_trusted_proxy_test.go`，覆盖默认不信任 forwarded headers、受信任代理解析真实客户端 IP、未受信任 peer 忽略 forwarded headers。
- [x] 2.3 检查认证 controller 和共享 HTTP middleware，确认继续统一通过 `c.ClientIP()` 获取 `client_ip`，不新增手写 header 解析分支。

## 3. 认证与观测测试

- [x] 3.1 更新 `user-service/internal/features/auth/transport/http/controller_test.go`，验证登录 use case 收到 Gin trusted proxy 解析后的 `authctx.ClientContext.ClientIP`。
- [x] 3.2 更新 `common/http/middleware` 日志相关测试，验证 access log 和认证失败日志的 `client_ip` 跟随 Gin trusted proxy 行为。
- [x] 3.3 运行相关包测试：`go test ./common/runtime/config ./common/http/middleware ./user-service/internal/providers ./user-service/internal/features/auth/transport/http`。

## 4. 文档与部署资产

- [x] 4.1 更新 `docs/ARCHITECTURE.md`、根 `README.md` 和 `user-service/README.md`，将“应用不接受 trusted proxy 配置”改为 `server.http.trusted_proxies` 显式可信代理策略。
- [x] 4.2 更新 Kubernetes、Helm、Compose 或 Nacos 配置示例与部署 README，说明生产环境必须按入口拓扑配置 trusted proxy CIDR，并要求入口层覆盖或重建 forwarded headers。
- [x] 4.3 运行 `make user-service-architecture-lint`，验证架构和文档约束未产生 drift。

## 5. 最终验证

- [x] 5.1 运行 `openspec status --change configure-trusted-proxies`，确认 proposal、design、specs 和 tasks 均为完成状态。
- [x] 5.2 检查本次预期变更 diff，确认没有 OpenAPI、Ent、migration 或无关文件 drift。
- [x] 5.3 暂存本次预期代码、文档和 OpenSpec artifact 变更。
- [x] 5.4 运行 `make lint`。
- [x] 5.5 运行 `make verify`。
