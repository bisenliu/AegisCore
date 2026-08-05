// Package client 提供基于 Resty 的业务中立出站 HTTP 请求能力。
//
// # 请求配置
//
// NewRequest 创建带 60 秒默认 timeout 和已初始化 query、form、header maps 的 SendRequest。
// SendRequest 零值也可使用：Timeout 为零时仍使用 DefaultTimeout，nil maps 表示没有对应参数。
// URL 和 Method 裁剪首尾空白后必须非空。基本 JSON 请求参见 ExampleSendRequest。
//
// QueryParams 会与 URL 自带 query 合并，Headers 作为 request-level header 传给 Resty。FormData
// 非空时优先于 JSONData，并由 Resty 编码为 application/x-www-form-urlencoded；否则非 nil
// JSONData 交给 Resty 按 body 类型和 Content-Type 编码。
//
// # Context 与 timeout
//
// SendContext 使用调用方 context，并通过派生 context 为单次请求应用 Timeout。调用方更早的
// deadline 或 cancellation 仍优先生效；负数 Timeout 在网络请求前返回 ErrInvalidTimeout。
// Send 是使用 context.Background 的便捷入口，不适合需要跟随请求或应用生命周期取消的调用路径。
// context 取消示例参见 ExampleSendRequest_SendContext。
//
// # Client 与代理所有权
//
// 未设置 RestyClient 和 ProxyURL 时，package 复用长期存活的默认 Resty client。默认 client 使用
// Go 安全 TLS 校验，不保存 cookie，不启用 retry、debug logging、认证或 tracing。它的 transport
// 连接池由 package 生命周期拥有，调用方不得修改或关闭。
//
// RestyClient 非 nil 时由调用方拥有，helper 不修改其 client-level 配置；已有 middleware、retry、
// transport、TLS、redirect 和 response body limit 继续生效。固定或高频代理应配置在调用方长期复用
// 的 client 上。便捷 ProxyURL 只允许在未注入 client 时使用，必须是绝对 HTTP(S) URL，并为单次发送
// 创建专用 client、在返回前关闭 idle connections。注入示例参见 ExampleSendRequest_injectedClient。
//
// retry 是否安全取决于 method、幂等键和上游协议，必须由消费侧集成定义；common 默认不重试。
// 自定义 CA 和 mTLS 也必须通过调用方拥有的 transport 配置，不能关闭 TLS 证书校验作为通用默认值。
//
// # 响应与错误
//
// 全部 2xx 响应返回 success=true、完整 body 和 nil error。非 2xx 响应返回 success=false、完整 body
// 和可通过 errors.As 检查的 *StatusError；StatusError 文本不包含 body。调用方应在所属 integration
// 边界解析 body 并映射业务错误。Resty 构造、middleware、body limit、context、TLS 或 transport
// 错误返回 success=false、nil body 和包装后的原始错误。状态错误示例参见 ExampleStatusError。
//
// # 快照与并发
//
// 每次 SendContext 都创建 request-level 浅层快照，复制 QueryParams、FormData 和 Headers，并在副本
// 上裁剪 URL、Method 与 ProxyURL、填充默认 timeout。发送不会修改调用方的 SendRequest 或 maps；
// 前一次发送返回后，可以修改同一 SendRequest 再顺序发送。
//
// 快照不会 deep-copy JSONData 中的 map、slice、pointer 或其他引用，也不会缓存或重放 io.Reader，
// 不会 clone RestyClient。调用方必须在发送返回前保持这些对象稳定；包含 io.Reader 的请求通常只能
// 发送一次，除非调用方显式重建 reader。同一个 SendRequest 不支持并发修改或并发发送；需要并发
// 请求时应分别创建配置对象，并确保共享 RestyClient 本身及其配置按调用方式保持并发安全。
//
// # 能力边界
//
// 本 package 只负责单次请求构造和发送，不拥有外部服务 DTO、认证、签名、retry policy、业务错误
// 映射、日志或防腐逻辑。这些能力应留在消费服务的 internal/integration/http 或所属 feature。
package client
