package config

import (
	"context"
	"fmt"

	commonconfig "github.com/aegiscore/common/runtime/config"
	commonnacos "github.com/aegiscore/common/runtime/config/nacos"
)

// LoadResult 包含已应用默认值的服务配置和原始配置来源信息。
type LoadResult struct {
	Config *Config
	Source commonconfig.SourceMetadata
}

// EffectiveSettings 从已应用默认值的 Config 生成最终生效配置，避免维护会漂移的重复状态。
func (r *LoadResult) EffectiveSettings() (map[string]any, error) {
	if r == nil || r.Config == nil {
		return nil, fmt.Errorf("encode runtime config: config is required")
	}
	return commonconfig.EncodeSettings(r.Config)
}

// Load 从 Nacos 分层配置加载 user-service 配置。
func Load(ctx context.Context) (*LoadResult, error) {
	env, err := commonnacos.LoadEnv()
	if err != nil {
		return nil, err
	}
	source, err := commonnacos.NewSource(env)
	if err != nil {
		return nil, err
	}
	settings, metadata, err := commonconfig.LoadSource(ctx, source)
	if err != nil {
		return nil, err
	}
	return DecodeSettings(settings, metadata)
}

// DecodeSettings 将合并后的配置 map 解码为 user-service Config。
func DecodeSettings(settings map[string]any, source commonconfig.SourceMetadata) (*LoadResult, error) {
	cfg, err := commonconfig.DecodeStrict(settings, commonconfig.DecodeOptions[Config]{
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
		digest, digestErr := commonconfig.DigestSettings(settings)
		if digestErr != nil {
			return nil, digestErr
		}
		source.Digest = digest
	}
	return &LoadResult{Config: cfg, Source: source}, nil
}

// NewRuntimeConfig 提供共享 runtime 配置，供 common provider 消费。
func NewRuntimeConfig(cfg *Config) *commonconfig.Config {
	runtime := cfg.RuntimeConfig()
	return &runtime
}
