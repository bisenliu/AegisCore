## ADDED Requirements

### Requirement: Migration naming guidance preserves history
数据库迁移相关命名标准化 SHALL 记录未来 migration 文件应使用清晰语义名称的约束，但不得重命名已存在 migration 文件、修改 `atlas.sum` 历史或改变数据库 schema。

#### Scenario: Existing migration filename is unclear
- **WHEN** 审查发现已存在 migration 文件名语义较泛
- **THEN** 实现 MUST 保留该文件名和迁移历史，并只在文档或规格中记录未来命名建议

#### Scenario: Migration capability is unaffected
- **WHEN** 命名标准化完成
- **THEN** Atlas migration 校验、生成脚本、apply 脚本和数据库结构 MUST 与修改前保持一致
