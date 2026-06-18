# 数据库迁移规格

## 需求

### 需求：Ent 与 Atlas 工作流
Ent schema 必须作为数据库结构来源。`user-service/migrations/` 下的 Atlas SQL migration 必须作为可审查迁移工件。

#### 场景：schema 变更
Given Ent schema 被修改
When 准备提交变更
Then 必须生成 Ent 代码和 Atlas migration，并审查 SQL 与 `atlas.sum`。

#### 场景：手动修改 SQL
Given 生成的 SQL 被手动调整
When 提交迁移
Then 必须刷新 `atlas.sum`，并确保 `make user-service-migrate-validate` 或模块级 `make -C user-service migrate-validate` 通过。

#### 场景：Atlas 配置位置
Given 执行用户服务迁移命令
When 调用 Atlas
Then 必须使用 `user-service/migrations/atlas.hcl` 和同目录下的 SQL migration、`atlas.sum`；迁移应用必须只执行已提交 migration，不得在运行时生成 schema。

### 需求：运行时不修改 schema
运行时服务代码不得使用 `client.Schema.Create(ctx)` 表达 schema 变更。

#### 场景：E2E 初始化 schema
Given E2E 测试需要数据库 schema
When 初始化 PostgreSQL
Then schema 必须来自 Atlas SQL migration，不得使用运行时自动建表。

### 需求：生产迁移顺序
生产迁移必须作为 CI/CD release job 或独立 migration Job 在 HTTP rollout 前执行。普通服务容器默认不得执行 migration。

#### 场景：容器启动
Given `RUN_MIGRATIONS` 为 false 或未设置
When user-service 容器启动
Then 必须直接启动服务，不得应用 migration。

#### 场景：显式兼容模式
Given `RUN_MIGRATIONS=true`
When 容器启动
Then entrypoint 可以在启动 HTTP server 前执行 migration，但该模式只适用于简单部署或兼容场景。

#### 场景：生产发布顺序
Given 生产或多副本环境发布 user-service
When 执行数据库和 RBAC 初始化
Then 推荐顺序必须是 migration Job、RBAC seed Job、HTTP server rollout；普通服务副本不得通过默认 entrypoint 竞争执行 migration。
