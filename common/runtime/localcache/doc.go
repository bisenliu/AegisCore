// Package localcache 提供有容量上限、固定 TTL 和并发回源合并的进程内 loading cache。
//
// # 配置
//
// Config 的配置语义如下：
//
//   - Name 是缓存实例的稳定名称，会裁剪首尾空白且不能为空。它适合用作 metrics 中的低基数 cache
//     label，不应包含 user ID、请求 ID 等动态内容。
//   - Capacity 是最多保留的 item 数量，必须大于零。它限制 item 数量，不表示 value 的内存字节数。
//   - TTL 是 value 从成功写入缓存开始计算的固定有效期，必须大于零。命中 Get 不会延长 TTL，因此
//     localcache 不是 sliding expiration cache。
//   - LoadTimeout 是一次共享 loader 调用的最长时间，必须大于零。超时通过 context 通知 loader，
//     loader 必须主动检查 ctx.Done() 或把 ctx 传给下游调用；缓存不会强制终止 goroutine。
//   - loader 在缓存 miss 时按 key 回源，不能为空。成功结果会写入缓存，错误会原样返回且不会缓存。
//
// key 不会被裁剪、解析或规范化。调用方应在 Get 和 Invalidate 前生成同一种稳定字符串，例如同一 UUID
// 不要混用大小写或带空白的形式。Get 接受 nil context，并将其视为 context.Background；请求路径通常
// 应传入真实 context，使调用方可以停止等待。
//
// # 读取与固定 TTL
//
// 第一次读取 miss key 时会调用 loader，后续命中直接返回缓存值。TTL 从成功写入开始计算，即使期间
// 持续命中，值仍会在固定 TTL 后过期。完整构造和读取代码参见 ExampleLoadingCache。
//
// LoadingCache 的公开方法可以由多个 goroutine 并发调用，不需要 Start、Close 或后台清理生命周期。
// 过期 item 在读取时逻辑失效，并在后续成功写入前惰性清理。
//
// # 并发 miss 与调用方取消
//
// 同一时刻有多个 goroutine Get 同一个 miss key 时，只执行一次 loader，所有仍在等待的调用方共享该
// 结果；不同 key 的 loader 可以并发执行。每个公开 Get 分别记录一次 miss，但共享回源成功或失败只
// 记录一次 load result。
// 同 key 并发 miss 的可执行代码参见 ExampleLoadingCache_Get_concurrentMiss。
//
// loader 使用最先启动本轮回源的调用方 context 作为 value 来源，但会通过 context.WithoutCancel
// 移除该 context 原有的取消和 deadline，再施加 Config.LoadTimeout。某个调用方超时或取消时，它自己
// 的 Get 会立即返回 ctx.Err()，但不会取消其他调用方正在共享的回源。即使所有调用方都已取消，已经
// 开始的 loader 仍可运行到完成或 LoadTimeout，并在成功时填充缓存。
//
// loader 不应把 context 之外的请求局部对象带到异步生命周期中，也不应依赖某个调用方的 deadline；
// 需要限制数据库、Redis 或 HTTP 请求时，应把收到的 loader context 继续传给下游。
//
// # 强失效
//
// Invalidate 同步删除指定 key；InvalidateAll 同步删除当前全部 item，适合配置整体切换等少数场景。
// 二者都不强制停止已经运行的 loader，而是通过实例级 revision 禁止失效前启动的回源结果重新发布旧值。
//
// 业务写入成功后应主动失效对应 key，完整代码参见 ExampleLoadingCache_Invalidate。
//
// revision 属于整个 cache。一次 Invalidate(key) 只删除该 key 的已缓存 value，但也会抑制同一实例中
// 更早开始、尚未发布的其他 key 回源结果。如果第一次回源与失效冲突，Get 会自动再回源一次；如果第
// 二次回源也再次遭遇失效，则返回 ErrInvalidated，由上层选择失败、稍后重试或重新读取。受控复现连续
// 失效冲突的代码参见 ExampleLoadingCache_Get_invalidationConflict。
//
// Invalidate 应在权威数据写入成功后调用；如果先失效后写入，两个操作之间的读取仍可能再次缓存旧值。
// InvalidateAll 的影响范围更大，频繁调用会降低命中率并抑制同时进行的回源，不能替代精确 key 失效。
//
// # 错误与超时
//
//   - loader 返回的数据库、Redis 或业务错误会直接返回给当前共享调用方，LoadError 加一；该错误不会
//     进入缓存，下一次 Get 会重新回源。
//   - loader 超过 LoadTimeout 且正确响应 context 时，通常返回 context.DeadlineExceeded，并计入
//     LoadError。
//   - 调用方 context 先取消时，该 Get 返回调用方的 context.Canceled 或
//     context.DeadlineExceeded；共享 loader 不因此取消，其最终结果仍决定 LoadSuccess 或 LoadError。
//   - ErrInvalidated 表示连续两次成功回源都因并发失效而没有发布，不是 loader 自身错误。
//
// # 统计
//
// Stats 返回从实例创建至今的累计快照，不会重置计数。每个公开 Get 单独计入 Hit 或 Miss；共享 loader
// 成功或失败分别计入 LoadSuccess 或 LoadError。LoadSuccess 包含后来因 revision 冲突而未发布的结果。
//
// CapacityEvictions 只统计达到容量上限造成的驱逐，并在导致驱逐的 Get 返回前同步可见；TTL 到期、
// Invalidate 和 InvalidateAll 不增加该计数。Capacity 返回配置容量。各字段分别使用原子计数，并发读写
// 安全，但在高并发更新期间不保证所有字段对应完全相同的时间点。LoadingCache 实现 StatsSource，可
// 直接交给 metrics collector；指标 label 应使用 Name，不能使用原始 cache key。完整统计代码参见
// ExampleLoadingCache_Stats。
//
// # 可变 value 所有权
//
// LoadingCache 存储和返回 V 本身。若 V 是 pointer、map、slice 或内部包含可变引用，Get 不会 deep
// clone；任意调用方原地修改 value 都可能改变其他调用方看到的缓存内容，并产生 data race。应优先
// 缓存不可变 value；确需缓存可变结构时，在 loader 写入和对外返回的适当边界复制，并且不要把缓存内
// 对象直接交给会修改它的代码。slice 复制示例参见 ExampleLoadingCache_mutableValue。
package localcache
