## 1. 领域模型

- [x] 1.1 新增 `user-services/internal/domain/user.go`，定义 `domain.User` 并覆盖 Service 当前需要的用户资料、密码哈希、状态、token version 和时间戳字段。
- [x] 1.2 为 `domain.User` 增加状态规则方法，例如 `CanLogin()`、`RequiresPasswordChange()` 和改密状态校验方法，并复用现有 `UserStatus` 枚举语义。
- [x] 1.3 补充或更新 domain 单元测试，验证正常、禁用、必须改密状态下的登录和改密规则。

## 2. Repository 边界

- [x] 2.1 修改 `user-services/internal/repository/user_repository.go`，将 `Create`、`GetByUsername`、`GetByUserID` 和 `ListUsers` 返回类型从 Ent 用户模型改为领域用户模型。
- [x] 2.2 在 `user-services/internal/repository/postgres/user_repository.go` 添加 Ent 到 Domain 的 mapper，覆盖 `ID`、`UserID`、`Nickname`、`Username`、`PasswordHash`、`Status`、`TokenVersion`、`CreatedAt` 和 `UpdatedAt`。
- [x] 2.3 更新 PostgreSQL repository 的创建、按用户名查询、按外部 user_id 查询和列表查询方法，确保 Ent 类型只存在于 `repository/postgres` 包内。
- [x] 2.4 保持 `GetTokenVersion`、`IncrementTokenVersion` 和 `UpdateCredentials` 的领域错误语义不变，确认未找到用户仍返回 `domain.ErrUserNotFound`。
- [x] 2.5 更新 repository 测试，验证创建、查询、列表和唯一约束错误转换仍符合现有行为。

## 3. Service 去 Ent 化

- [x] 3.1 更新 `user-services/internal/service/user_service.go`，移除 Ent import，并将用户响应 mapper 改为接收 `domain.User`。
- [x] 3.2 更新 `UserService.CreateUser`、`GetUserByID` 和 `ListUsers`，保持密码 hash、UUIDv7 生成、用户名冲突映射、not found 映射和分页响应语义不变。
- [x] 3.3 更新 `user-services/internal/service/auth_service.go`，移除 Ent import，使 `authenticateUser` 返回 `domain.User` 并通过领域方法判断登录和必须改密状态。
- [x] 3.4 更新改密流程，使用户状态校验基于领域用户模型，同时保持凭证更新、token version 递增和 Redis token version 缓存失效语义不变。
- [x] 3.5 检查 `user-services/internal/service` 包不得再导入 `github.com/aegiscore/user-services/ent`。

## 4. 测试与验证

- [x] 4.1 更新 service 层测试 stub，将 `ent.User` 构造替换为 `domain.User` 构造。
- [x] 4.2 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 4.3 在 `user-services/` 运行 `go test ./...`，修复编译或行为回归。
- [x] 4.4 在 `common/` 运行 `go test ./...`，确认共享模块未受影响。
- [x] 4.5 确认本变更未修改 Ent schema、Ent 生成代码、Atlas migration、Redis key 格式、HTTP API 路径或响应 DTO 字段。
