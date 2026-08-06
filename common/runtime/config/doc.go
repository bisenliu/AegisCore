// Package config 提供跨服务共享、业务中立的 runtime 配置原语。
//
// 固定加载管线从 DocumentSource 开始：调用方先通过 LoadSource 读取一组有序
// ConfigDocument，包内按文档顺序执行 DeepMergeYAML，并在 defaults 和 normalize
// 之前基于 raw merged settings 计算 SourceMetadata.Digest。随后调用方使用
// DecodeStrict 显式传入 DecodeOptions：Defaults 提供完整默认值，Normalize 可按
// raw 配置做服务内归一化，Validate 负责最终校验。DecodeStrict 会在 Normalize 和
// Validate 之前拒绝未知配置键，并报告完整叶子路径。
//
// Config 只包含 app、runtime、server、log 和 observability 这些服务无关配置组。
// 服务需要扩展 auth、RBAC、Ent、rate limit、具名资源必需名称或其他业务字段时，
// 应在服务私有配置类型中通过 mapstructure:",squash" 嵌入 Config，并由服务显式
// 组合 defaults、normalize 和 validate。common 不自动发现服务 hook，也不内置服务
// 私有敏感路径策略。
//
// 成功得到 typed config 后，调用方可以用 EncodeSettings 将有效配置编码为基于
// mapstructure tag 的 map，并将 time.Duration 输出为可读字符串。展示配置前，调用方
// 应按自身策略调用 RedactSettings 生成脱敏副本，再用 RenderYAML 渲染 YAML。
package config
