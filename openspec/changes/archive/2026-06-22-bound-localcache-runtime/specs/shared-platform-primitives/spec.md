## ADDED Requirements

### Requirement: 有界本地缓存 primitive

系统 MUST 在 `common/runtime/localcache` 中提供有明确容量上限、TTL、回源合并、主动失效、统计快照和关闭语义的本地缓存 primitive。缓存实例 MUST 通过显式配置创建，配置 MUST 包含名称、容量、TTL 和 key string 编码；旧的仅传入 TTL 的构造方式 MUST 被移除。

#### Scenario: 创建有界本地缓存

- **WHEN** 服务创建 `localcache` 实例
- **THEN** 系统 MUST 要求配置 `Name`、正数 `Capacity`、正数 `TTL` 和 `KeyString`
- **AND** 系统 MUST 将 `Capacity` 作为本地缓存容量预算，第一版以 `cost=1` 表示最大条目预算

#### Scenario: 拒绝无效缓存配置

- **WHEN** 服务使用空名称、非正数容量、非正数 TTL、空 key string 编码或空 loader 创建 loading cache
- **THEN** 系统 MUST 返回明确错误并拒绝创建缓存

#### Scenario: 缓存读取与回源

- **WHEN** 调用方通过 `GetOrLoad` 读取 key 且本地缓存 miss
- **THEN** 系统 MUST 使用 `singleflight` 合并同 key 并发回源
- **AND** 系统 MUST 在回源成功后尝试写入本地缓存
- **AND** 系统 MUST NOT 缓存 loader 返回的错误

#### Scenario: 缓存对象隔离

- **WHEN** 缓存 value 类型可被调用方修改
- **THEN** 系统 MUST 允许调用方提供 `CloneFunc`
- **AND** 系统 MUST 使用 clone 隔离 loader 返回对象、缓存内部对象和调用方返回对象

#### Scenario: 关闭后的访问

- **WHEN** 缓存实例已经调用 `Close`
- **THEN** `GetOrLoad` 和 `Set` MUST 返回 `ErrClosed`
- **AND** `Get` MUST 返回未命中
- **AND** `Delete` 和 `Clear` MUST 不再触碰底层缓存

#### Scenario: 统计不污染命中率

- **WHEN** `GetOrLoad` 进入 singleflight 后执行 double-check 并命中缓存
- **THEN** 系统 MUST 记录 double-check 命中
- **AND** 系统 MUST NOT 将该内部命中计入业务 hit 统计
