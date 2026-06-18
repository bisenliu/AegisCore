# 仓库架构规格

## 需求

### 需求：Go workspace 模块归属

AegisCore 必须保持为包含 `common` 与 `user-service` 两个模块的 Go workspace。仓库根目录不得被当作单一 Go module。

#### 场景：运行 Go 命令

Given 协作者需要构建、测试、lint 或生成代码
When 执行 Go 相关命令
Then 必须使用根目录 Makefile 入口，或在 `common/`、`user-service/` 模块目录内执行模块级命令。

#### 场景：根 Makefile 服务私有命令

Given 某个 Makefile 目标只作用于单个服务
When 该目标暴露在仓库根 Makefile
Then 目标名必须使用服务名前缀，例如 `user-service-seed-rbac`，不得使用 `seed-rbac` 这类无服务上下文的名称。

### 需求：模块边界

`common` 必须只承载跨服务稳定契约和无业务语义基础能力。`user-service` 必须拥有服务运行时、业务 feature、Ent schema、migration、服务侧 OpenAPI 生成和服务内 shared kernel。`deployments` 必须拥有部署与观测资产。

#### 场景：新增共享逻辑

Given 某段逻辑只服务于 user-service 的请求清洗、DTO 映射、授权、会话策略或持久化
When 选择代码位置
Then 该逻辑必须留在 `user-service` 内，不得放入 `common`。

#### 场景：新增部署文件

Given 协作者新增 Docker、Compose、Kubernetes、Helm、dashboard 或 alert 文件
When 选择目录
Then 文件必须放在 `deployments/` 下，不得移动到 `common`、`internal/shared` 或 feature 代码目录。

### 需求：OPSX 有效来源

当前有效 OPSX/OpenSpec 工件必须位于 `docs/opsx/` 和 `openspec/`。

### 需求：规格语言约束

所有定义仓库、模块、feature、分层、契约、runtime、部署、集成、可观测性、测试、质量门禁或 OpenAPI 边界的 OpenSpec 主规格、后续 change artifact 和 OPSX 相关文档必须使用简体中文。包名、路径、HTTP method、配置 key、CLI 命令、Go symbol、错误码、数据库字段等技术标识符可以保留英文原文。

#### 场景：生成边界相关规格

Given 代理或协作者需要创建或更新边界生成相关 spec
When 写入 `openspec/specs/` 或 `openspec/changes/<change>/` 下的规格 artifact
Then 正文、标题、需求、场景和任务说明必须使用简体中文，不得默认生成英文模板内容。

#### 场景：更新 OPSX 文档

Given 代理或协作者需要更新 OPSX 工作流、能力地图或其他 OpenSpec 相关文档
When 写入 `docs/opsx/` 或 `openspec/config.yaml`
Then 面向协作者阅读的说明性文本必须使用简体中文，并明确保留后续生成和更新时的中文要求。
