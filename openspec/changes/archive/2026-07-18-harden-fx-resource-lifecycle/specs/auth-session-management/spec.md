## ADDED Requirements

### Requirement: Auth 主动资源延迟启动与显式关闭

系统 MUST 在 auth Fx composition 中把 session purge pool、token-version 本地缓存等主动后台资源作为 auth feature 自有资源管理。会启动 goroutine、worker 或内部清理循环的资源 MUST 优先在 `OnStart` 创建，在 `OnStop` 关闭，并 MUST 先于共享 Redis client 关闭。

#### Scenario: Auth worker pool 启动失败回滚
- **WHEN** auth session purge pool 已在 `OnStart` 创建
- **AND** 后续 auth 或服务级启动 hook 失败导致 App 启动失败
- **THEN** purge pool MUST 被停止并 drain 已接收任务
- **AND** 停止过程 MUST 不关闭共享 Redis client

#### Scenario: Auth local cache 生命周期
- **WHEN** token-version 本地缓存启用并在启动阶段创建
- **THEN** 服务停止或启动失败时 MUST 显式关闭该缓存
- **AND** disabled 或 direct 模式 MUST 继续提供幂等 no-op close 语义

#### Scenario: Auth constructor 部分失败清理
- **WHEN** auth composition 中必须在 constructor 阶段创建多个部分资源
- **AND** 后续资源创建、配置选择或 wiring 失败
- **THEN** 已创建且归 auth 拥有的资源 MUST 立即关闭
- **AND** 关闭失败 MUST 与原始失败一起返回或记录

### Requirement: Auth 生命周期调整不改变认证语义

系统 MUST 在调整 auth 资源生命周期时保持登录、refresh、强制改密、退出、token version 校验和安全撤销语义不变。

#### Scenario: 资源 holder 不产生安全降级
- **WHEN** auth 资源尚未启动、启动失败或已关闭
- **THEN** 受保护访问和会话撤销流程 MUST 返回明确错误或保持 fail-closed
- **AND** 系统 MUST NOT 因 holder 中资源为空而允许旧 token、无效 refresh session 或撤销不完整结果通过
