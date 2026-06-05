## 1. 接口定义调整

- [x] 1.1 在 `user-services/internal/repository/user_repository.go` 中将现有用户仓储抽象拆分为用户资料、用户凭证和 token version 三个小接口。
- [x] 1.2 将用户资料接口限定为 `Create`、`GetByUserID` 和 `ListUsers` 等 `userService` 实际消费的方法。
- [x] 1.3 将用户凭证接口限定为认证凭证读取和凭证更新所需方法，避免包含用户列表或 token version 递增等无关能力。
- [x] 1.4 将 token version 接口限定为 `GetTokenVersion` 和 `IncrementTokenVersion` 等会话控制实际消费的方法。
- [x] 1.5 评估是否保留旧 `UserRepository` 类型；若保留，仅作为过渡组合接口，不允许新消费方继续依赖。

## 2. 实现层与依赖注入适配

- [x] 2.1 确认 `repository/postgres` 现有用户仓储结构体的方法集合同时满足三个小接口，且不拆散现有结构体和方法。
- [x] 2.2 调整 Fx provider，使同一个 PostgreSQL 用户仓储实例可以分别以三个小接口身份注入。
- [x] 2.3 检查 Fx 装配不因多接口导出而重复创建多个用户仓储实例或重复初始化底层数据库依赖。
- [x] 2.4 保持 `repository` 根包不依赖 `repository/postgres`，保持 controller/service/repository 分层边界不变。

## 3. 服务构造函数收敛

- [x] 3.1 调整 `NewUserService` 和 `userService` 字段，使用户资料服务只依赖用户资料仓储接口。
- [x] 3.2 调整认证服务 Fx 参数，使凭证组件接收用户凭证仓储接口，而不是完整用户仓储大接口。
- [x] 3.3 调整认证会话组件构造，使其接收 token version 仓储接口和 `AuthSessionRepository`。
- [x] 3.4 检查 `AuthService` 仍只保存凭证、token、会话组件和必要编排策略，不直接持有完整用户仓储。
- [x] 3.5 确认登录、改密、刷新、退出当前设备和退出全部设备的外部行为保持不变。

## 4. 测试替身收敛

- [x] 4.1 收敛用户资料服务测试 fake，使其只实现创建、按 ID 查询和列表查询相关方法。
- [x] 4.2 收敛登录认证测试 fake，使其只实现凭证读取和凭证更新相关方法。
- [x] 4.3 收敛 token version 测试 fake，使其只实现读取和递增 token version 相关方法。
- [x] 4.4 删除测试中为满足旧大接口而保留的无关空方法、panic 方法或不可达分支。
- [x] 4.5 补充或更新编译期断言，确保 PostgreSQL 用户仓储实现满足新的小接口。

## 5. 旧接口清理与验证

- [x] 5.1 搜索并清理服务层、认证组件和测试中对旧完整 `UserRepository` 的直接依赖。
- [x] 5.2 运行 `gofmt` 格式化变更过的 Go 文件。
- [x] 5.3 在 `user-services/` 运行 `go test ./...` 验证服务模块。
- [x] 5.4 在 `common/` 运行 `go test ./...` 确认共享模块未受影响。
- [x] 5.5 确认本变更未修改 Ent schema、生成代码、Atlas migration、HTTP 路由、响应契约或配置项。
