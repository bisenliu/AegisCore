# Permission Feature Skeleton

本目录用于预留未来 permission feature 边界。

当出现明确业务需求时，权限定义、权限查询、权限边界规则、HTTP transport、持久化 adapter，或与 role use case 的 feature-local 协作应放在这里。

当前范围：

- 不实现权限业务逻辑。
- 不新增 HTTP route 或 controller。
- 不新增 application、domain、infrastructure 或 Fx package。
- 不新增 Ent schema、生成代码、migration、seed data 或数据库表。
- 不修改认证或授权行为。

后续实现时，按仓库 feature 分层规则扩展：

- `application/`：承载 use case 编排和 feature-owned ports。
- `domain/`：承载权限实体、值对象、错误和纯领域规则。
- `transport/http/`：承载权限 HTTP DTO、validation、controller 和 route。
- `infrastructure/*`：承载持久化或外部 adapter 实现。
- `fx.go`：承载 feature-local provider wiring。
