## MODIFIED Requirements

### Requirement: runtime dependency 观测使用稳定资源名

系统 MUST 使用稳定低基数资源名标识 runtime dependency 指标、健康检查和告警。user-service 主 PostgreSQL runtime dependency 的资源名 MUST 为 `primary_db`，Redis 缓存资源名保持 `cache_redis`。

#### Scenario: PostgreSQL runtime dependency 暴露观测标签

- **WHEN** user-service 注册 PostgreSQL 连接池指标、健康检查或告警查询
- **THEN** PostgreSQL 资源名 MUST 使用 `primary_db`
- **AND** 指标 label、健康检查名称、dashboard 查询和 alert 表达式 MUST 保持一致
