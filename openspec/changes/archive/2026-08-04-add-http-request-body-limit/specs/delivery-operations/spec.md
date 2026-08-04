## ADDED Requirements

### Requirement: HTTP 请求体上限配置与部署默认值

系统 MUST 在 user-service 运行配置和 Nacos 环境资产中声明 HTTP 入站请求体字节上限。默认值 MUST 与当前 Pod 内存预算相容，并 MUST 在代码默认值与本地 Nacos 配置之间保持一致；配置非法时服务 MUST 在启动前失败。Compose 与 Helm MUST 继续只负责选择 Nacos 配置来源，不得为该业务字段新增专用环境变量覆盖。

#### Scenario: 服务配置声明请求体上限

- **WHEN** user-service 加载 `server.http` 或等价服务私有 HTTP 配置
- **THEN** 系统 MUST 应用请求体最大字节数默认值
- **AND** 零值或负值等非法上限 MUST 在启动前失败并报告配置字段路径

#### Scenario: Nacos 环境资产同步默认值

- **WHEN** 发布本地 Nacos 配置并通过 Compose 或 Helm 启动 user-service
- **THEN** Nacos 服务配置 MUST 声明与代码默认值一致的请求体上限
- **AND** Compose 与 Helm MUST 只传递 Nacos 来源选择配置，不得通过专用环境变量覆盖请求体上限

#### Scenario: 发布与回滚

- **WHEN** 发布包含请求体上限的 user-service 版本
- **THEN** 发布 MUST 不需要数据库 migration、RBAC seed 变化或 OpenAPI 重新生成
- **AND** 如合法请求被误拒，运维 MUST 能通过提高配置上限并滚动重启回滚行为
