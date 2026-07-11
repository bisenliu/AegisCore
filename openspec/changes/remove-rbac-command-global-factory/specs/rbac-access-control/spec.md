## ADDED Requirements

### Requirement: RBAC CLI 显式依赖装配

系统 MUST 在 RBAC CLI 引导命令中使用单次命令调用范围内的显式依赖工厂装配 seed、超级管理员绑定和超级管理员创建所需资源。RBAC CLI MUST NOT 依赖可变 package-level 工厂来注入运行时依赖或测试替身。

#### Scenario: 生产命令使用显式依赖工厂

- **WHEN** user-service 构造 `rbac seed`、`rbac assign-super-admin` 或 `rbac create-super-admin` 命令
- **THEN** 命令 runner MUST 通过显式参数获得依赖工厂
- **AND** 真实依赖工厂 MUST 保持既有配置加载、数据库连接、Ent client 创建和 cleanup 顺序

#### Scenario: 测试命令使用局部替身

- **WHEN** RBAC command 测试需要注入依赖替身
- **THEN** 测试 MUST 通过局部 runner、局部命令对象或局部依赖参数传入替身
- **AND** 测试 MUST NOT 写入 package-level 可变依赖工厂

#### Scenario: RBAC 业务行为保持不变

- **WHEN** RBAC seed、超级管理员绑定或超级管理员创建命令在依赖创建成功后执行
- **THEN** 系统 MUST 保持既有 RBAC seed、超级管理员绑定和超级管理员创建业务语义
- **AND** 系统 MUST 保持既有命令超时、输出文本和 cleanup 调用语义
