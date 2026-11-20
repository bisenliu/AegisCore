package config

import (
	"context"
	"fmt"

	platformconfig "github.com/aegiscore/platform/runtime/config"
	platformnacos "github.com/aegiscore/platform/runtime/config/nacos"
)

// LoadResult 包含已应用默认值的服务配置和原始配置来源信息。
type LoadResult struct {
	Config *Config
	Source platformconfig.SourceMetadata
}

// EffectiveSettings 从已应用默认值的 Config 生成最终生效配置，避免维护会漂移的重复状态。
func (r *LoadResult) EffectiveSettings() (map[string]any, error) {
	if r == nil || r.Config == nil {
		return nil, fmt.Errorf("encode runtime config: config is required")
	}
	return platformconfig.EncodeSettings(r.Config)
}

// Load 从 Nacos 分层配置加载 user-service 配置。
func Load(ctx context.Context) (*LoadResult, error) {
	env, err := platformnacos.LoadEnv()
	if err != nil {
		return nil, err
	}
	source, err := platformnacos.NewSource(env)
	if err != nil {
		return nil, err
	}
	settings, metadata, err := platformconfig.LoadSource(ctx, source)
	if err != nil {
		return nil, err
	}
	return DecodeSettings(settings, metadata)
}

// DecodeSettings 将合并后的配置 map 解码为 user-service Config。
func DecodeSettings(settings map[string]any, source platformconfig.SourceMetadata) (*LoadResult, error) {
	cfg, err := platformconfig.DecodeStrict(settings, platformconfig.DecodeOptions[Config]{
		Defaults: DefaultConfig,
		Normalize: func(cfg *Config) {
			normalizeConfig(cfg, settings)
		},
		Validate: Config.Validate,
	})
	if err != nil {
		return nil, err
	}
	if source.Digest == "" {
		digest, digestErr := platformconfig.DigestSettings(settings)
		if digestErr != nil {
			return nil, digestErr
		}
		source.Digest = digest
	}
	return &LoadResult{Config: cfg, Source: source}, nil
}

// NewRuntimeConfig 提供共享 runtime 配置，供 platform provider 消费。
func NewRuntimeConfig(cfg *Config) *platformconfig.Config {
	runtime := cfg.RuntimeConfig()
	return &runtime
}
