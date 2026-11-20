package config

import (
	"context"
	"fmt"
	"strings"
)

// SourceMetadata 描述一次成功配置加载的来源和摘要。
type SourceMetadata struct {
	Provider  string
	Service   string
	Namespace string
	Group     string
	DataIDs   []string
	Digest    string
}

// DataIDsCSV 返回稳定的文档标识列表文本。
func (m SourceMetadata) DataIDsCSV() string {
	return strings.Join(m.DataIDs, ",")
}

// DocumentSource 按来源声明的稳定顺序加载配置文档及来源信息。
type DocumentSource interface {
	LoadDocuments(context.Context) ([]ConfigDocument, SourceMetadata, error)
}

// LoadSource 加载并合并来源文档，并基于默认值和归一化前的 raw settings 计算摘要。
func LoadSource(ctx context.Context, source DocumentSource) (map[string]any, SourceMetadata, error) {
	if source == nil {
		return nil, SourceMetadata{}, fmt.Errorf("load config source: document source is required")
	}
	docs, metadata, err := source.LoadDocuments(ctx)
	if err != nil {
		return nil, SourceMetadata{}, fmt.Errorf("load config source: %w", err)
	}
	settings, err := DeepMergeYAML(docs)
	if err != nil {
		return nil, SourceMetadata{}, err
	}
	digest, err := DigestSettings(settings)
	if err != nil {
		return nil, SourceMetadata{}, err
	}
	metadata.DataIDs = append([]string(nil), metadata.DataIDs...)
	metadata.Digest = digest
	return settings, metadata, nil
}
