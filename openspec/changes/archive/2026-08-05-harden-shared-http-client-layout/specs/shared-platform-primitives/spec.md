## ADDED Requirements

### Requirement: 出站 HTTP 请求配置快照与所有权

系统 MUST 在每次发送边界持有独立、归一化且通过校验的 request-level 配置快照，并 MUST 明确只做浅层复制；共享 helper 不得为并发便利隐式 deep-copy 业务 body、clone 注入 client 或改变调用方拥有对象的生命周期。

#### Scenario: 创建逐次发送快照

- **WHEN** 调用方通过 `Send` 或 `SendContext` 发送包含 query、form 或 header maps 的 `SendRequest`
- **THEN** helper MUST 在 Resty 请求构造前复制这些 maps，并使用裁剪首尾空白后的 URL、method 和 proxy URL 形成该次发送快照
- **AND** helper MUST NOT 修改调用方持有的 `SendRequest` 或其中的 maps

#### Scenario: 顺序复用请求配置

- **WHEN** 前一次发送已经返回且调用方修改同一个 `SendRequest` 后再次发送
- **THEN** 后一次发送 MUST 使用后一次调用开始时的配置，不得复用前一次 Resty request 的 query、form 或 header 状态

#### Scenario: body 与注入 client 所有权

- **WHEN** `JSONData` 包含 map、slice、pointer、`io.Reader` 或其他引用值，或者调用方提供 `RestyClient`
- **THEN** helper MUST 将这些值视为调用方拥有的浅层引用，不得隐式 deep-copy、缓存、重放或 clone
- **AND** 调用方 MUST 在发送返回前保持 body 与注入 client 配置稳定，并 MUST NOT 并发修改或并发发送同一个 `SendRequest`
