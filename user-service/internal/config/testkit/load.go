package testkit

import (
	"context"

	platformconfig "github.com/aegiscore/platform/runtime/config"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// LoadFromDocuments 使用测试提供的多段 YAML 生成 user-service 配置。
func LoadFromDocuments(docs []platformconfig.ConfigDocument) (*serviceconfig.LoadResult, error) {
	settings, source, err := platformconfig.LoadSource(context.Background(), documentSource{docs: docs})
	if err != nil {
		return nil, err
	}
	return serviceconfig.DecodeSettings(settings, source)
}

type documentSource struct {
	docs []platformconfig.ConfigDocument
}

// LoadDocuments 返回输入文档 slice 的副本，避免配置加载过程持有或修改测试调用方的 slice。
func (s documentSource) LoadDocuments(context.Context) ([]platformconfig.ConfigDocument, platformconfig.SourceMetadata, error) {
	docs := append([]platformconfig.ConfigDocument(nil), s.docs...)
	dataIDs := make([]string, 0, len(docs))
	for _, doc := range docs {
		dataIDs = append(dataIDs, doc.DataID)
	}
	return docs, platformconfig.SourceMetadata{Provider: "testkit", DataIDs: dataIDs}, nil
}
