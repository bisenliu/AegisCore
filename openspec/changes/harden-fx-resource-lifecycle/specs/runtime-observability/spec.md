## ADDED Requirements

### Requirement: Fx tracing 资源启动失败安全

系统 MUST 在 Fx 装配 tracing provider 时避免 constructor 阶段创建带后台副作用的 tracing batch processor 或 exporter；真实 tracing 运行资源 MUST 在 `OnStart` 中创建，并 MUST 在 `OnStop` 中关闭。禁用 tracing 时 MUST 保持非 nil no-op 语义且不得连接 exporter。

#### Scenario: 后续启动失败不泄漏 tracing 资源
- **WHEN** tracing 已启用且 Fx App 在 tracing `OnStart` 成功后因后续 hook 失败而启动失败
- **THEN** App MUST 关闭 tracing `OnStart` 创建的 provider、batch processor 和 exporter
- **AND** 关闭错误 MUST 被保留或记录为可诊断信息，不得静默吞掉

#### Scenario: constructor 阶段无后台副作用
- **WHEN** Fx graph 构造 tracing provider 依赖但 App 尚未执行 `Start`
- **THEN** tracing constructor MUST NOT 启动 batch processor、建立 OTLP exporter 连接或注册需要 stop hook 才能清理的后台资源
- **AND** 无效静态配置仍 MUST 在构造或启动阶段返回明确错误

### Requirement: Ent 观测 wrapper 生命周期安全

系统 MUST 保持 Ent 查询 tracing 和 metrics 观测语义不变，并确保 Ent wrapper 或观测资源在启动失败路径中不会依赖未执行的 stop hook 才完成清理。

#### Scenario: Ent wrapper 部分构造失败回滚
- **WHEN** user-service 构造 Ent client wrapper 或其观测依赖时后续步骤失败
- **THEN** 已创建且需要关闭的部分资源 MUST 立即关闭
- **AND** 返回错误 MUST 保留原始构造失败和清理失败信息
