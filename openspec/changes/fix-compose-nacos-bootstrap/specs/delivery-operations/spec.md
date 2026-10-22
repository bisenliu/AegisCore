## ADDED Requirements

### Requirement: Compose Nacos 就绪与管理员初始化

本地 Compose MUST 使用 Nacos 3.x readiness 接口判断 Nacos 是否可供依赖任务使用。配置发布任务 MUST 在管理员凭据初始化或校验成功后启动；已有管理员密码 MUST NOT 被初始化任务重置。

#### Scenario: 新数据卷首次启动

- **WHEN** Nacos 已就绪且数据卷尚无管理员用户
- **THEN** Compose MUST 使用 `NACOS_PASSWORD` 初始化 `nacos` 管理员并验证登录
- **AND** 两个 Nacos 配置发布任务 MUST 在初始化成功后运行

#### Scenario: 已有管理员数据卷

- **WHEN** Nacos 数据卷已有管理员用户
- **THEN** Compose MUST 使用配置的凭据验证登录并保持原密码不变
- **AND** 凭据不匹配时配置发布任务 MUST NOT 启动

#### Scenario: Nacos 尚未就绪

- **WHEN** Nacos readiness 接口未返回成功状态
- **THEN** Compose MUST NOT 启动管理员初始化和配置发布任务
