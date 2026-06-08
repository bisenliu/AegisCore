## ADDED Requirements

### Requirement: Refresh orchestration remains readable and strategy-oriented

用户会话控制能力 SHALL 保持 `AuthService.Refresh` 的高层用例编排清晰。刷新流程 MUST 将请求规范化、Refresh Token claims 解析、Refresh 会话校验、非轮转刷新、轮转刷新和轮转失败处理表达为职责明确的内部方法或组件边界。`AuthService.Refresh` MUST NOT 直接堆叠 Redis 旧会话校验、新会话创建、旧会话删除和补偿清理的低层细节。

#### Scenario: Refresh method selects a refresh strategy

- **Given** 调用方提交 Refresh Token 请求
- **When** `AuthService.Refresh` 处理请求
- **Then** 系统 MUST 先完成请求规范化、Refresh Token claims 解析和 Refresh 会话校验
- **Then** 系统 MUST 根据 Refresh Token 轮转配置选择非轮转刷新或轮转刷新策略
- **Then** 轮转策略的会话写入和撤销细节 MUST 位于职责明确的内部辅助方法、session lifecycle 组件或 repository 抽象内

#### Scenario: Non-rotation refresh keeps current session semantics

- **Given** Refresh Token 轮转未启用
- **Given** Refresh Token 校验通过且 Redis 中存在对应会话
- **When** 调用方请求刷新 token
- **Then** 系统 MUST 复用当前 `session_id` 签发新的 Access Token 和 Refresh Token
- **Then** 系统 MUST 保持现有响应信封、错误映射、JWT claims 和 Redis key 格式不变

### Requirement: Refresh token rotation consumes old sessions atomically

启用 Refresh Token 轮转时，系统 SHALL 将旧 Refresh 会话仍有效的校验、新 Refresh 会话创建、用户会话索引更新和旧 Refresh 会话撤销作为一个原子提交动作执行。该原子动作 MUST 在 Redis repository 或等价持久化边界内实现，并 MUST 覆盖多 goroutine、多进程和多服务实例并发刷新场景。系统 MUST NOT 依赖服务进程内互斥锁作为主要重放防护机制。

#### Scenario: Concurrent rotation succeeds once for the same old refresh token

- **Given** Refresh Token 轮转已启用
- **Given** Redis 中存在旧 Refresh Token 对应的会话记录
- **When** 两个或多个请求并发使用同一个旧 Refresh Token 执行刷新
- **Then** 系统 MUST 最多只允许一个请求完成旧会话消费并返回新的 Refresh Token
- **Then** 其他请求 MUST 因旧会话已不存在、已被消费或会话状态不匹配而失败
- **Then** Redis 中 MUST NOT 同时保留多个由同一个旧 Refresh Token 并发轮转成功产生的新会话

#### Scenario: Atomic rotation leaves no split-brain session state

- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **When** 系统提交 Refresh Token 轮转
- **Then** 旧会话存在性校验、新会话写入、用户会话索引写入、旧会话删除和旧索引移除 MUST 作为 Redis Lua 脚本、Redis 事务或等价机制中的一个原子动作完成
- **Then** 系统 MUST 避免因命令间失败造成旧会话和新会话同时可用或同时不可用的中间状态

#### Scenario: Rotation failure does not expose an unusable new refresh token

- **Given** Refresh Token 轮转已启用
- **Given** 新 token 已在内存中签发
- **Given** Redis 原子轮转提交失败、旧会话已被其他请求消费或旧会话状态不匹配
- **When** 系统处理刷新响应
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 向调用方返回该新 Refresh Token
- **Then** 该失败 MUST NOT 破坏已经由其他请求成功提交的会话状态

#### Scenario: Token signing failure keeps old refresh session usable

- **Given** Refresh Token 轮转已启用
- **Given** Refresh Token 校验通过
- **Given** 新 token 签发失败
- **When** 调用方请求刷新 token
- **Then** 系统 MUST 返回失败响应
- **Then** 系统 MUST NOT 消费或撤销已通过校验的旧 Refresh 会话
- **Then** 调用方后续 MAY 使用旧 Refresh Token 重试刷新
