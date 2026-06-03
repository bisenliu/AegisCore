## 1. Session Store ZSet 重构

- [x] 1.1 修改 `user-services/internal/service/session_store.go` imports，移除 `fmt.Sscan` 依赖并新增 `strconv`，确认 `fmt` 仍仅用于 key 格式化和错误包装。
- [x] 1.2 在 `CreateSession` 中将用户会话索引写入从 `SAdd` 改为 `ZAdd`，member 使用 `session.SessionID`，score 使用实际会话过期 Unix 秒。
- [x] 1.3 在 `CreateSession` 的 Redis transaction pipeline 中加入 `ZRemRangeByScore(userSessionsKey(session.UserID), "-inf", nowUnix)`，并移除索引 Key 的主要 `Expire` 清理依赖。
- [x] 1.4 在 `DeleteSession` 中将 `SRem` 改为 `ZRem`，并在同一个 Redis transaction pipeline 中加入 `ZRemRangeByScore` 清理该用户已过期索引 member。
- [x] 1.5 在 `DeleteAllUserSessions` 中将 `SMembers` 改为先 `ZRemRangeByScore` 清理过期 member，再读取未过期 `session_id`，最后批量删除对应 `auth:session:<session_id>` 并删除用户索引 Key。
- [x] 1.6 如存在重复的过期清理 score 格式化逻辑，提取 service 内私有 helper，保持 `SessionStore` 公共接口不变。

## 2. Token Version 热路径优化

- [x] 2.1 将 `parseTokenVersion(value string)` 实现替换为 `strconv.ParseInt(value, 10, 64)`。
- [x] 2.2 确认 `GetCurrentTokenVersion` 对非法缓存值的现有语义不变：解析失败或非正数时回源 PostgreSQL，Redis 非 `Nil` 错误仍返回错误。

## 3. 测试更新

- [x] 3.1 更新 `user-services/internal/service/session_store_test.go` 中会话索引断言，从 Set 断言改为 ZSet 断言，验证 member 为 `session_id`。
- [x] 3.2 新增或更新创建会话测试，验证 ZSet score 等于实际会话过期 Unix 时间戳，且创建时会清理 score 小于或等于当前时间的旧 member。
- [x] 3.3 新增或更新退出当前设备测试，验证目标 session key 被删除、ZSet 中目标 member 被移除、已过期 member 同步被清理。
- [x] 3.4 新增或更新退出全部设备测试，验证先清理过期 member，只对未过期 indexed sessions 执行删除，并最终删除用户会话索引 Key。
- [x] 3.5 新增或保留 token version 解析测试，覆盖合法十进制 int64、非法值回源和非正数回源行为。

## 4. 验证

- [x] 4.1 对修改后的 Go 文件运行 `gofmt -w`。
- [x] 4.2 在 `user-services/` 模块运行 `go test ./...`。
- [x] 4.3 如果测试暴露 common 依赖影响，在 `common/` 模块补充运行 `go test ./...`。
- [x] 4.4 确认未修改 controller、repository、HTTP API、Ent schema、Atlas migration 或生成代码。
