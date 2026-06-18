# 用户资料规格

## 需求

### 需求：用户资料操作
user feature 必须支持创建用户、按 UUID 查询用户和分页列出用户。

#### 场景：创建用户
Given 授权客户端提交有效创建用户输入
When 调用 `POST /api/v1/users`
Then 服务必须持久化用户，并生成唯一对外 `user_id` UUID、用户名、密码哈希、状态和时间戳。

#### 场景：查询用户详情
Given 用户已存在
When 调用 `GET /api/v1/users/:user_id`
Then 响应必须使用 UUID `user_id` 标识用户，不得暴露内部数字数据库 ID。

#### 场景：分页列出用户
Given 系统中存在用户
When 调用 `GET /api/v1/users` 并携带分页参数
Then 结果必须使用 common pagination 契约。

### 需求：用户功能分层
用户写侧用例必须位于 `application/command`，读侧用例必须位于 `application/query`，协议无关输入辅助位于 `application/validators`，HTTP DTO 位于 `transport/http`，Ent 访问位于 `infrastructure/postgres`。

#### 场景：新增用户读侧用例
Given 需要新增用户读侧操作
When 添加 application 代码
Then 查询实现必须位于用户功能的 `application/query`，并消费用户功能 application 拥有的端口。
