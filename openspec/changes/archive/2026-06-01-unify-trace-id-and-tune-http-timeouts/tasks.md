## 1. Trace 标识统一

- [x] 1.1 检查 `common/middleware/trace_id.go` 与 `common/logger/context.go` 的 trace 常量和 context 读写路径，确认 `X-Trace-ID`、`trace_id`、`trace-id` 分别只用于 HTTP header、Gin context key 和日志字段。
- [x] 1.2 如发现重复生成、字段名漂移或日志字段不一致，最小化调整共享 `common` 实现，保持对外 header 与日志字段兼容。
- [x] 1.3 为 trace-id 中间件和 logger context API 增加或补齐测试，覆盖请求 header 传入、缺失生成、非法值替换，以及日志字段使用同一 trace 值。

## 2. HTTP Timeout 配置

- [x] 2.1 修改 `user-services/configs/config.yaml`，将 `http.read_timeout` 设置为 `30s`、`http.write_timeout` 设置为 `60s`、`http.idle_timeout` 设置为 `120s`、`http.shutdown_timeout` 设置为 `25s`。
- [x] 2.2 检查用户服务 HTTP server 创建逻辑，确认 read、write、idle timeout 和 graceful shutdown timeout 均来自加载后的配置值，且可被 `AEGISCORE_` 环境变量覆盖。
- [x] 2.3 补齐配置加载或 HTTP runtime 测试，验证默认 YAML timeout 值被正确反序列化，并保持配置 loader 不执行额外 required 或范围校验。

## 3. 验证

- [x] 3.1 运行 `gofmt -w` 格式化修改过的 Go 文件。
- [x] 3.2 在 `common/` 执行 `go test ./...`。
- [x] 3.3 在 `user-services/` 执行 `go test ./...`。
- [x] 3.4 复查实现是否未改动 HTTP API envelope、数据库、Redis、Ent schema 或 Atlas migration。
