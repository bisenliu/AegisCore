## 1. 配置模型与加载

- [x] 1.1 在 `common/config.Config` 增加 `SystemConfig`，包含 `Timezone string` 并映射 `system.timezone`。
- [x] 1.2 更新 `common/config` 加载测试，覆盖 YAML 加载 `system.timezone`、`AEGISCORE_SYSTEM_TIMEZONE` 覆盖，以及缺失字段不报错。
- [x] 1.3 更新 `user-services/configs/config.yaml`，增加带注释的 `system.timezone: Asia/Shanghai` 示例配置。

## 2. 共享 Timezone Module

- [x] 2.1 新增 `common/timezone` 包，提供基于 `*config.Config` 的初始化函数和 Fx `Module`。
- [x] 2.2 初始化逻辑默认使用 `Asia/Shanghai`，配置非空时使用配置值，成功后设置 `time.Local` 和 `TZ`。
- [x] 2.3 初始化逻辑使用进程级 once 保护，重复触发时保持第一次成功初始化结果。
- [x] 2.4 无效时区返回包含配置值或底层 `time.LoadLocation` 原因的错误，不使用 panic。
- [x] 2.5 为 `common/timezone` 增加单元测试，覆盖默认时区、配置时区、无效时区、重复初始化和环境状态恢复。

## 3. 用户服务接入

- [x] 3.1 在 `user-services/internal/bootstrap` 中显式引入共享 timezone module，保持 `common/infrastructure.Module` 不隐式修改所有调用方的全局时区。
- [x] 3.2 增加或调整 bootstrap/Fx 装配测试，验证用户服务启动图包含 timezone 初始化，且不额外创建未声明 datastore 依赖。
- [x] 3.3 确认 timezone 接入不修改 HTTP 路由、响应信封、Ent schema 或 Atlas migration。

## 4. 验证与整理

- [x] 4.1 对修改过的 Go 文件运行 `gofmt -w`。
- [x] 4.2 在 `common/` 执行 `go test ./...`。
- [x] 4.3 在 `user-services/` 执行 `go test ./...`。
- [x] 4.4 如测试暴露全局时区状态污染，调整测试隔离而不是移除生产 once 语义。
