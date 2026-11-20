package config

import (
	"context"
	"fmt"
	"strings"
)

// Example_pipeline 展示业务中立配置管线的典型调用顺序：先合并文档，
// 再加载 source metadata 与 raw digest，最后执行严格解码、编码和 YAML 渲染。
func Example_pipeline() {
	docs := []ConfigDocument{
		{DataID: "base.yaml", Content: []byte(`app:
  name: example-service
server:
  http:
    port: 28080
`)},
		{DataID: "service.yaml", Content: []byte(`log:
  level: debug
`)},
	}
	_, _ = DeepMergeYAML(docs)

	source := exampleDocumentSource{
		docs: docs,
		metadata: SourceMetadata{
			Provider: "memory",
			Service:  "example-service",
			DataIDs:  []string{"base.yaml", "service.yaml"},
		},
	}
	settings, metadata, err := LoadSource(context.Background(), source)
	if err != nil {
		panic(err)
	}
	cfg, err := DecodeStrict(settings, DecodeOptions[Config]{
		Defaults: DefaultConfig,
		Validate: Config.Validate,
	})
	if err != nil {
		panic(err)
	}
	encoded, err := EncodeSettings(cfg)
	if err != nil {
		panic(err)
	}
	rendered, err := RenderYAML(encoded)
	if err != nil {
		panic(err)
	}

	fmt.Println(metadata.DataIDsCSV())
	fmt.Println(strings.HasPrefix(metadata.Digest, "sha256:"))
	fmt.Println(cfg.Server.HTTP.Port)
	fmt.Println(strings.Contains(string(rendered), "level: debug"))

	// Output:
	// base.yaml,service.yaml
	// true
	// 28080
	// true
}

type exampleDocumentSource struct {
	docs     []ConfigDocument
	metadata SourceMetadata
}

// LoadDocuments 实现示例用内存文档来源，模拟真实 source 返回文档和元数据。
func (s exampleDocumentSource) LoadDocuments(context.Context) ([]ConfigDocument, SourceMetadata, error) {
	return s.docs, s.metadata, nil
}
