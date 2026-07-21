# AegisCore 产品上下文

## 1. 项目目标

AegisCore 是面向后端服务的 Go workspace 项目底座，当前聚焦用户服务和跨服务基础能力。它提供用户资料、认证会话、角色、权限、RBAC 授权、健康探针、OpenAPI、metrics、tracing、logging、Ent/Atlas migration 和部署观测资产，使后续服务可以在稳定契约和运行时基础上演进。

## 2. 用户与角色

| 角色 | 目标 | 关注点 |
|---|---|---|
| 终端业务用户 | 使用账号登录并访问受保护业务能力 | 登录可靠性、会话安全、权限正确性 |
| 系统管理员 | 管理用户、角色、角色绑定和超级管理员账号 | RBAC seed、授权策略一致性、审计和恢复能力 |
| 后端开发者 | 在既有分层内新增或修改服务能力 | 共享契约、代码生成、测试、OpenAPI 和 migration 流程 |
| 运维和平台协作者 | 部署、观测和发布 user-service | 配置、健康检查、metrics、dashboard、migration 和发布顺序 |
| AI 代理和协作者 | 通过 OPSX/OpenSpec 安全推进变更 | 能力地图、主规格、change artifacts、验证命令 |

## 3. 核心场景

1. 用户通过认证接口登录，系统校验凭证、用户状态和会话策略后签发 token。
2. 已认证用户携带 bearer token 访问用户、角色或权限接口，系统先校验 token version，再执行 RBAC 授权。
3. 开发者在 `rbacbaseline.DefaultPermissions()` 维护权限定义；管理员创建用户和角色，并动态维护角色权限及用户角色绑定。
4. 运维执行 RBAC seed、创建或绑定超级管理员，并按发布顺序执行 migration 与服务 rollout。
5. 开发者修改 API、schema、观测或共享契约时，先通过 OPSX change 说明影响，再按 tasks 实施和验证。

## 4. 当前能力范围

| 能力 | 说明 |
|---|---|
| `shared-platform-primitives` | 跨服务契约、HTTP helper、runtime primitive、安全原语、测试基础设施 |
| `user-identity-management` | 用户资料创建、查询、列表、状态约束和受保护 HTTP 边界 |
| `auth-session-management` | 登录、刷新、退出、改密、会话和 token version 校验 |
| `rbac-access-control` | 代码定义的权限目录只读投影、角色、角色权限、用户角色、Casbin 授权、RBAC seed |
| `runtime-observability` | 健康检查、OpenAPI、metrics、OTLP tracing、stdout/stderr logging、独立 pprof、dashboard 和 alerts |
| `delivery-operations` | 构建、测试、lint、OpenAPI 生成、Ent/Atlas migration、Docker、Compose、Kubernetes、Helm |
| `opsx-foundation` | 仓库级 OPSX/OpenSpec 文档、配置、能力地图、主规格和变更工作流 |

## 5. 关键约束

- 认证、授权和密码处理必须优先保证安全，不泄露凭证匹配细节。
- API 响应、错误、分页和校验错误优先复用 `common/contract/` 与 `common/http/`。
- 生产发布先执行 user-service `primary_db` migration，再执行 RBAC seed，最后启动或滚动 HTTP 副本。
- 权限删除必须由受控 migration 先清理角色绑定再删除权限；seed 只按稳定 `permission_id` upsert，不自动删除权限。
- OpenSpec 主规格和 OPSX 文档必须保持简体中文，技术标识符可保留原文。
- 生成物必须可验证，OpenAPI、Ent 和 dashboard 相关变更需要 drift 检查。
