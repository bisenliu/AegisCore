package nacos

import (
	"bytes"
	"context"
	"fmt"

	commonconfig "github.com/aegiscore/common/runtime/config"
)

type documentLoader interface {
	LoadConfigDocument(ctx context.Context, env Env, dataID string) ([]byte, error)
}

// Source 从 Nacos 按声明顺序加载配置文档。
type Source struct {
	env    Env
	loader documentLoader
}

// NewSource 创建 Nacos document source，并在加载前校验服务地址。
func NewSource(env Env) (*Source, error) {
	loader, err := newV3Loader(env, nil)
	if err != nil {
		return nil, err
	}
	return newSource(env, loader), nil
}

func newSource(env Env, loader documentLoader) *Source {
	env.DataIDs = append([]string(nil), env.DataIDs...)
	return &Source{env: env, loader: loader}
}

// LoadDocuments 拉取全部 Nacos 文档，并返回不含 raw digest 的来源元数据。
func (s *Source) LoadDocuments(ctx context.Context) ([]commonconfig.ConfigDocument, commonconfig.SourceMetadata, error) {
	if s == nil || s.loader == nil {
		return nil, commonconfig.SourceMetadata{}, fmt.Errorf("load nacos config: source is not initialized")
	}
	docs := make([]commonconfig.ConfigDocument, 0, len(s.env.DataIDs))
	for _, dataID := range s.env.DataIDs {
		content, err := s.loader.LoadConfigDocument(ctx, s.env, dataID)
		if err != nil {
			return nil, commonconfig.SourceMetadata{}, fmt.Errorf(
				"load nacos config %s/%s/%s: %w",
				s.env.Namespace,
				s.env.Group,
				dataID,
				err,
			)
		}
		if len(bytes.TrimSpace(content)) == 0 {
			return nil, commonconfig.SourceMetadata{}, fmt.Errorf(
				"load nacos config %s/%s/%s: document is empty or not found",
				s.env.Namespace,
				s.env.Group,
				dataID,
			)
		}
		docs = append(docs, commonconfig.ConfigDocument{DataID: dataID, Content: content})
	}
	return docs, commonconfig.SourceMetadata{
		Provider:  "nacos",
		Service:   s.env.Service,
		Namespace: s.env.Namespace,
		Group:     s.env.Group,
		DataIDs:   append([]string(nil), s.env.DataIDs...),
	}, nil
}

var _ commonconfig.DocumentSource = (*Source)(nil)
