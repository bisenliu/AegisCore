## ADDED Requirements

### Requirement: localcache 泛型结果与同步容量统计

系统 MUST 让 `LoadingCache[V]` 对其声明支持的全部泛型 value 保持一致的成功加载语义，并 MUST 在公开读取返回前同步完成由本次发布引起的容量驱逐统计；实现不得为无关删除注册异步 eviction callback。

#### Scenario: 接口类型 loader 返回 nil

- **WHEN** `V` 是接口类型且 loader 成功返回 nil value
- **THEN** `Get` MUST 返回 nil 和 nil error，不得因 singleflight 的 `any` 结果缺少动态类型而 panic
- **AND** 后续同 key `Get` MUST 命中已缓存的 nil value，不得重复回源

#### Scenario: 新值触发容量驱逐

- **WHEN** cache 已达到配置容量且成功发布一个此前不存在的新 key
- **THEN** cache MUST 驱逐一个 item，并在本次 `Get` 返回前同步增加 `Stats.CapacityEvictions`

#### Scenario: 非容量删除不产生异步驱逐统计

- **WHEN** item 因 TTL 清理、`Invalidate` 或 `InvalidateAll` 被移除
- **THEN** `Stats.CapacityEvictions` MUST 保持不变
- **AND** localcache MUST NOT 依赖为每个删除 item 启动 goroutine 的 eviction callback 来维护该统计
