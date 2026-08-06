## 1. Common 脱敏原语

- [x] 1.1 全仓搜索 `RedactSettings`、`RenderYAML`、`auth.jwt.secret` 和 password 路径调用点，确认需要迁移的 common 与 user-service 边界。
- [x] 1.2 修改 `common/runtime/config` redaction 实现，删除内置默认敏感路径，使调用方显式路径成为唯一脱敏策略来源。
- [x] 1.3 保持并完善 deep clone 行为，覆盖 map、slice、多层嵌套、nil settings、nil 路径、空路径、未知路径和 `*` 通配路径处理。
- [x] 1.4 更新 common 单元测试，确保 common 生产代码和测试不再出现 `auth.jwt.secret` 或其他 feature 私有路径。

## 2. user-service 调用方策略

- [x] 2.1 在 `user-service/internal/config` 或 CLI 渲染边界集中声明 user-service 敏感路径列表，覆盖 JWT、Redis、PostgreSQL 及当前服务私有敏感字段。
- [x] 2.2 更新 `user-service/cmd config render` 和配置测试调用方，显式使用 user-service 敏感路径策略调用 `common/runtime/config.RedactSettings`。
- [x] 2.3 补充 user-service render 测试，验证 JWT secret、Redis password 和 PostgreSQL password 不出现在输出中，且原 settings map 不被修改。
- [x] 2.4 确认不修改 Nacos dataId、环境变量名、运行时配置结构、YAML merge、strict decode、raw digest 或配置来源加载流程。

## 3. 规格与验证

- [x] 3.1 运行相关 Go 测试，至少覆盖 `common/runtime/config` 和 user-service config/CLI 相关包。
- [x] 3.2 运行 `make user-service-architecture-lint`，确认 common 与 user-service 边界符合架构约束。
- [x] 3.3 暂存本次预期代码、测试和 OpenSpec artifact 变更，避免最终 verify 被未暂存预期 diff 阻塞。
- [x] 3.4 运行 `make lint` 并修复所有失败。
- [x] 3.5 运行 `make verify` 并修复所有失败；如生成物 drift 出现，运行对应生成命令后重新验证。
