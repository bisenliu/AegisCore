## 1. Baseline Review And Tests

- [x] 1.1 梳理 `common/` 当前公开 API、现有测试和 `user-services` 引用点，记录必须保持兼容的函数、类型和默认行为。
- [x] 1.2 为 `common/response.Fail(nil)`、JSON trailing body、默认 logger 并发读写、Redis ping timeout 和 trace-id 输入边界补充失败优先测试。
- [x] 1.3 运行 `go test ./...` 于 `common/`，确认新增测试能暴露当前缺口或在实现前被明确标记为待修复用例。

## 2. Configuration And Infrastructure

- [x] 2.1 保持 `common/config.Load` 不执行 required、range、字段存在性校验或默认值填充，并确认本 change 不新增相关 API。
- [x] 2.2 为 `RedisConfig` 增加 ping timeout 配置，更新 Redis provider 使用实例配置值而非固定 `5*time.Second`。
- [x] 2.3 增加显式 opt-in 的命名 Redis/PostgreSQL Fx provider helper，并测试 helper 只连接调用方声明的实例。

## 3. Middleware And Response Contract

- [x] 3.1 在 `common/middleware` 内集中维护 trace-id header、context key、logger key、CORS header 和默认 method/header 常量。
- [x] 3.2 增加可配置 CORS options，支持 origins、methods、headers、exposed headers、credentials、max age 和 origin reflection 的 `Vary: Origin`。
- [x] 3.3 增加可配置 trace-id options，支持最大长度、合法字符或格式校验，并在非法入站 trace id 时生成替代值或按配置拒绝。
- [x] 3.4 在 `common/response` 集中维护成功消息、内部错误消息和业务码常量，移除重复硬编码。
- [x] 3.5 加固 `response.Fail` 和 `response.FromError` 的 nil error 路径，确保返回 HTTP 500、业务码 `90000` 和 `internal server error` 信封且不 panic。

## 4. Validation Refactor And Hardening

- [x] 4.1 将 `common/validation/validation.go` 拆分为 `module.go`、`validator.go`、`binder.go`、`errors.go`、`fields.go`、`translations.go` 或等价聚焦文件，保持公开 API 和现有行为不变。
- [x] 4.2 在 `common/validation` 中集中维护请求 tag 名称、自定义 `enum` rule 名称和默认校验消息，更新 binder、字段名解析和翻译注册复用这些常量。
- [x] 4.3 加固 JSON binder，拒绝首个 JSON 值之后的额外 token 或额外 JSON 值，并补充对应 HTTP 400 失败信封测试。
- [x] 4.4 为 JSON binder 增加 opt-in strict unknown-field 选项，默认保持兼容行为，启用后返回可读 binding 错误。
- [x] 4.5 扩展 URI/query/form 反射 binder，支持 `encoding.TextUnmarshaler`、`time.Duration` 和匿名嵌入结构体指针字段，并为不支持类型返回可读错误。

## 5. Logger Refactor And Concurrency

- [x] 5.1 将 `common/logger/logger.go` 拆分为 context helper、factory、writer、daily writer 和 level helper 等聚焦文件，保持公开 API 和日志输出契约不变。
- [x] 5.2 使用 `sync.RWMutex`、`atomic.Value` 或等价机制保护默认 logger 的读写，消除 `SetDefault`、`FromContext` 和快捷日志函数的并发数据竞争。
- [x] 5.3 评估无 trace-id context 的日志字段策略，保持主规格要求的空字符串或明确生成值，并补充对应测试。

## 6. Verification

- [x] 6.1 对改动过的 Go 文件运行 `gofmt -w`。
- [x] 6.2 在 `common/` 运行 `go test ./...`，确认共享层测试通过。
- [x] 6.3 在 `user-services/` 运行 `go test ./...`，确认共享 API 兼容服务侧引用。
- [x] 6.4 如修改了 OpenSpec artifacts 或主规格归档前内容，运行对应 OpenSpec 校验命令并修复格式问题。
- [x] 6.5 复查 `common/` 文件组织，确认没有引入跨包 constants dumping ground、自动连接未声明 datastore 或业务层重复实现共享能力。
