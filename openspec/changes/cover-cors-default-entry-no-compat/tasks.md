## 1. 测试实现

- [x] 1.1 在 `common/http/middleware` 同包测试中补充 `CORS()` 默认入口覆盖，验证默认响应头、`OPTIONS` 预检 `204` 短路、普通请求继续进入业务 handler，以及与 `CORSWithOptions(defaultCORSOptions)` 的相关响应结果一致。
- [x] 1.2 补齐 `CORSWithOptions` 自定义策略已有稳定字段断言，覆盖 allow methods、allow headers、exposed headers、origin 反射、credentials 和 max age。
- [x] 1.3 检查新增或修改的 CORS 测试断言风格，确保常见断言使用语义化 `require` 或允许边界内的 `assert`，不新增机械 `Fail`/`Failf` 替换或旧兼容 helper。

## 2. 验证

- [x] 2.1 运行 `openspec validate cover-cors-default-entry-no-compat`，确认 change artifacts 合法。
- [x] 2.2 运行 `go test -cover ./common/http/middleware`。
- [x] 2.3 运行 `go test -coverprofile=<profile> ./common/http/middleware` 和 `go tool cover -func <profile>`，确认 `CORS` 有覆盖，且 `CORSWithOptions`、`normalizeCORSOptions` 覆盖率不降低。
- [x] 2.4 暂存本次预期变更后运行 `make lint` 和 `make verify`；如被非本次变更或环境条件阻塞，记录阻塞原因和已完成的替代验证。
