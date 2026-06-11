# HTTP Integration

`integration/http` 用于用户服务访问外部 HTTP API 的 client adapter。

可以放置：

- 外部 HTTP request/response DTO。
- 外部状态码、错误体和网络错误到 feature application/domain 错误的转换。
- per-system base URL、timeout、auth header、idempotency key、retry/backoff 等调用边界。
- 对 feature application port 的实现。

禁止放置：

- 本服务 Gin controller、route registration 或 Swagger DTO。
- `common/http/response` 输出逻辑。
- Ent、SQL、Redis store 或本服务持久化 adapter。
- 尚无真实调用方的 order/payment 等预设 client。

当前没有真实外部 HTTP 调用，因此本目录只保留 README 占位。
