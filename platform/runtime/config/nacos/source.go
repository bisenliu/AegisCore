package nacos

import (
	"bytes"
	"context"
	"fmt"

	platformconfig "github.com/aegiscore/platform/runtime/config"
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

// newSource 创建远程配置加载源，记录连接参数与拉取工具，后续用于从配置中心下载配置文件。
func newSource(env Env, loader documentLoader) *Source {
	env.DataIDs = append([]string(nil), env.DataIDs...)
	return &Source{env: env, loader: loader}
}

// LoadDocuments 拉取全部 Nacos 文档，并返回不含 raw digest 的来源元数据。
func (s *Source) LoadDocuments(ctx context.Context) ([]platformconfig.ConfigDocument, platformconfig.SourceMetadata, error) {
	if s == nil || s.loader == nil {
		return nil, platformconfig.SourceMetadata{}, fmt.Errorf("load nacos config: source is not initialized")
	}
	docs := make([]platformconfig.ConfigDocument, 0, len(s.env.DataIDs))
	for _, dataID := range s.env.DataIDs {
		content, err := s.loader.LoadConfigDocument(ctx, s.env, dataID)
		if err != nil {
			return nil, platformconfig.SourceMetadata{}, fmt.Errorf(
				"load nacos config %s/%s/%s: %w",
				s.env.Namespace,
				s.env.Group,
				dataID,
				err,
			)
		}
		if len(bytes.TrimSpace(content)) == 0 {
			return nil, platformconfig.SourceMetadata{}, fmt.Errorf(
				"load nacos config %s/%s/%s: document is empty or not found",
				s.env.Namespace,
				s.env.Group,
				dataID,
			)
		}
		docs = append(docs, platformconfig.ConfigDocument{DataID: dataID, Content: content})
	}
	return docs, platformconfig.SourceMetadata{
		Provider:  "nacos",
		Service:   s.env.Service,
		Namespace: s.env.Namespace,
		Group:     s.env.Group,
		DataIDs:   append([]string(nil), s.env.DataIDs...),
	}, nil
}

var _ platformconfig.DocumentSource = (*Source)(nil)
