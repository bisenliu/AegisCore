# 部署资产规格

## 需求

### 需求：部署资产归属
部署资产必须按类型放在 `deployments/`：Docker 放在 `deployments/docker`，Compose 放在 `deployments/compose`，Kubernetes 放在 `deployments/k8s`，Helm 放在 `deployments/helm`，观测资产放在 `deployments/observability`。

#### 场景：构建 Docker 镜像
Given 需要构建 user-service 镜像
When 执行 Docker build
Then 必须使用仓库根目录作为 build context，并指定 `deployments/docker/user-service.Dockerfile`。

#### 场景：Dockerfile 路径约束
Given 调整 user-service Dockerfile、entrypoint 或 COPY 规则
When 构建镜像
Then 路径必须继续以仓库根 build context 为基准，容器内迁移脚本和 Atlas 配置必须与 `user-service/migrations/` 当前布局兼容。

### 需求：Compose 本地运行时
Compose 可以提供本地 PostgreSQL、Redis、migration one-shot、RBAC seed、user-service、Prometheus 和 Grafana wiring。

#### 场景：Compose 启动顺序
Given 使用本地 Compose
When 服务启动
Then migration 必须先于 RBAC seed 执行，RBAC seed 必须先于 user-service app 启动。

#### 场景：生产发布顺序
Given 发布生产或多副本环境
When 编排 migration、RBAC seed 和 user-service rollout
Then 必须先执行 migration Job，再执行 RBAC seed Job，最后滚动发布 HTTP server；seed 不得替代运行中副本的 policy refresh。

### 需求：Kubernetes 和 Helm 当前为占位边界
当前 Kubernetes 与 Helm 目录只声明边界，不代表已经存在生产可用 manifests 或 chart。

#### 场景：新增生产 Kubernetes 支持
Given 需要新增 Kubernetes manifests
When 建模数据库迁移
Then migration 必须是 Deployment rollout 前的独立 Job，服务副本默认不得设置 `RUN_MIGRATIONS=true`。

#### 场景：占位目录约束
Given Kubernetes 或 Helm 目录当前只用于声明边界
When 新增部署资产
Then 不得为了填充目录而新增未验证的 Deployment、Service、Ingress、Secret 或 chart template；生产可用资产必须伴随对应验证方式和发布顺序说明。
