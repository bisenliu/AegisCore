// Package nacos 提供基于 Nacos v3 HTTP API 的配置文档来源适配器。
//
// 该 package 只负责从已解析的环境选择中读取 Nacos 文档，并实现
// config.DocumentSource 契约；配置合并、摘要、严格解码、默认值、
// normalize 和 validate 留在调用方显式组合的配置加载管线中。
package nacos
