## ADDED Requirements

### Requirement: Provide high-throughput HTTP timeout defaults

用户服务示例配置 MUST 提供适合较高请求量和 keep-alive 复用场景的 HTTP timeout 基线。默认 YAML 配置 MUST 设置 `http.read_timeout` 为 `30s`、`http.write_timeout` 为 `60s`、`http.idle_timeout` 为 `120s`、`http.shutdown_timeout` 为 `25s`。这些值 MUST 通过现有 YAML 与 `AEGISCORE_` 环境变量覆盖机制加载，HTTP server MUST 使用加载后的配置值。

#### Scenario: Load default HTTP timeouts from config
- **Given** 用户服务使用 `user-services/configs/config.yaml` 启动
- **When** 系统加载 HTTP runtime 配置
- **Then** `http.read_timeout` MUST 为 `30s`
- **Then** `http.write_timeout` MUST 为 `60s`
- **Then** `http.idle_timeout` MUST 为 `120s`
- **Then** `http.shutdown_timeout` MUST 为 `25s`

#### Scenario: HTTP server uses configured timeout values
- **Given** 配置加载得到 HTTP timeout 值
- **When** 用户服务创建 HTTP server 并执行 graceful shutdown
- **Then** HTTP server MUST 使用配置中的 read、write 和 idle timeout
- **Then** graceful shutdown MUST 使用配置中的 shutdown timeout

#### Scenario: Environment can override timeout defaults
- **Given** 部署环境通过 `AEGISCORE_` 前缀环境变量覆盖 HTTP timeout 配置
- **When** `common/config.Load` 反序列化运行时配置
- **Then** 用户服务 MUST 使用环境变量覆盖后的 timeout 值
- **Then** 配置加载器 MUST NOT 对 timeout 执行额外 required 或范围校验
