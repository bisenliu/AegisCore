# 共享内核规格

## 需求

### 需求：共享目录准入规则
`user-service/internal/shared` 只能承载至少两个真实功能消费、边界稳定且不能归入 `common` 的服务内业务内核。

#### 场景：新增共享 helper
Given helper 只被一个功能使用或只是技术工具函数
When 选择代码位置
Then 不得放入 `internal/shared`。

### 需求：当前允许包
当前共享包必须限制为 `identity` 和 `rbacbaseline`。新增共享包前必须同步更新 AGENTS、架构文档和相关规格，说明 owner、消费方、准入理由和禁止事项。

#### 场景：消费身份规格
Given user 和 auth 都需要用户状态或身份错误
When 引用这些概念
Then 必须消费 `internal/shared/identity`，不得复制常量或保留兼容 alias。

#### 场景：消费 RBAC 基线
Given role 和 permission 需要系统内置 RBAC 数据
When 执行 seed、路由差异诊断或策略加载
Then 必须消费 `internal/shared/rbacbaseline` 作为唯一基线来源。

#### 场景：shared 子包命名
Given 需要在 `internal/shared` 新增或调整共享业务内核
When 选择包名和文件名
Then 不得新增根级 `errors`、`enums`、`types`、`utils` 或 `helpers` 兜底包；公共错误必须放在 owning 子包的 `errors.go`，公共枚举文件必须按业务语义命名为 `<subject>_status.go`、`<subject>_type.go` 或 `<subject>_kind.go`。

#### 场景：禁止兼容 alias
Given 业务常量或稳定值对象已迁移到 shared 子包
When 功能代码使用该概念
Then 不得在功能内保留兼容 alias 或重复常量。

### 需求：共享目录禁止事项
共享包不得导入功能包、Gin、Ent、Redis、SQL、Fx、HTTP response helper、runtime provider，不得承载控制器、DTO、存储端口、用例、配置读取、日志副作用、外部调用、数据库/缓存访问或部署资产。

#### 场景：共享包需要数据库访问
Given 拟新增共享包需要 Ent、SQL 或 Redis
When 审查设计
Then 必须拒绝该共享包，或移动到所属功能的 application/infrastructure 边界。
