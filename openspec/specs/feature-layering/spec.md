# 功能分层规格

## 需求

### 需求：按功能组织业务代码

服务内业务代码必须位于 `user-service/internal/features/<feature>/`。不得新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。

#### 场景：新增业务用例

Given 用例属于 user、auth、role 或 permission 能力
When 开始实现
Then 代码必须放在所属功能包内，并按层组织。

### 需求：分层职责固定

功能内 `domain` 只承载纯领域规则；`application` 拥有用例和消费侧端口；`transport/http` 拥有 Gin DTO、控制器、路由和输入准备器；`infrastructure/postgres` 拥有 Ent/SQL 适配器；`infrastructure/redis` 拥有 Redis 适配器；`fx.go` 只做依赖装配。

#### 场景：处理 HTTP 请求

Given 控制器收到 HTTP 请求
When 准备用例输入
Then 必须先调用 `binding.BindOrAbort`，再调用一个功能内输入准备器，最后调用 application command/query。

#### 场景：HTTP 输入准备器边界

Given controller 已完成 `binding.BindOrAbort`
When 调用功能内输入准备器
Then 输入准备器只能执行裁剪、默认值归一化、UUID/cursor/token 解析和 application command/query 构造；不得查询 store、调用 use case、执行授权、写 HTTP 响应或导入 Ent、Redis、SQL、infrastructure adapter。

#### 场景：定义应用端口

Given 基础设施适配器需要持久化或加载数据
When 定义适配器实现的接口
Then 消费侧端口必须定义在功能 application 层，不得定义在 infrastructure 层。

#### 场景：保护 domain 依赖

Given 领域代码需要新增依赖
When 添加 import
Then 不得导入 Gin、Ent、Redis、config、logger、response envelope、application ports 或 infrastructure adapter。

### 需求：未来 transport 和事件目录按需创建

`transport/grpc`、`domain/events`、`domain/services` 和 `infrastructure/consumers` 只有存在真实 API、事件模型或消费者需求时才能承载业务代码。

#### 场景：当前没有真实 gRPC API

Given 服务当前没有入站 gRPC API
When 协作者触碰 `transport/grpc`
Then 只能保留 README 或包文档，不得新增空处理器、proto、生成代码或服务端运行时依赖。
