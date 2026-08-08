package testkit

import (
	"context"

	commonconfig "github.com/aegiscore/common/runtime/config"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// LoadFromDocuments 使用测试提供的多段 YAML 生成 user-service 配置。
func LoadFromDocuments(docs []commonconfig.ConfigDocument) (*serviceconfig.LoadResult, error) {
	settings, source, err := commonconfig.LoadSource(context.Background(), documentSource{docs: docs})
	if err != nil {
		return nil, err
	}
	return serviceconfig.DecodeSettings(settings, source)
}

type documentSource struct {
	docs []commonconfig.ConfigDocument
}

// LoadDocuments 返回输入文档 slice 的副本，避免配置加载过程持有或修改测试调用方的 slice。
func (s documentSource) LoadDocuments(context.Context) ([]commonconfig.ConfigDocument, commonconfig.SourceMetadata, error) {
	docs := append([]commonconfig.ConfigDocument(nil), s.docs...)
	dataIDs := make([]string, 0, len(docs))
	for _, doc := range docs {
		dataIDs = append(dataIDs, doc.DataID)
	}
	return docs, commonconfig.SourceMetadata{Provider: "testkit", DataIDs: dataIDs}, nil
}
