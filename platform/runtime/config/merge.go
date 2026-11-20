package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ConfigDocument 是配置来源按顺序返回的一份 YAML 文档。
type ConfigDocument struct {
	DataID  string
	Content []byte
}

// DeepMergeYAML 按文档顺序合并多段 YAML 配置。
func DeepMergeYAML(docs []ConfigDocument) (map[string]any, error) {
	merged := map[string]any{}
	for _, doc := range docs {
		var current map[string]any
		if len(doc.Content) == 0 {
			current = map[string]any{}
		} else if err := yaml.Unmarshal(doc.Content, &current); err != nil {
			return nil, fmt.Errorf("decode config document %s: %w", doc.DataID, err)
		}
		merged = mergeConfigValues(merged, current).(map[string]any)
	}
	return merged, nil
}

// mergeConfigValues 合并两份配置内容，后读取的配置会覆盖先前的同名配置；如果同一配置项下包含多条子项，则逐项更新子项，不会粗暴地把整组子项全部替换掉。
func mergeConfigValues(left any, right any) any {
	leftMap, leftOK := left.(map[string]any)
	rightMap, rightOK := right.(map[string]any)
	if leftOK && rightOK {
		out := make(map[string]any, len(leftMap)+len(rightMap))
		for key, value := range leftMap {
			out[key] = value
		}
		for key, value := range rightMap {
			if existing, ok := out[key]; ok {
				out[key] = mergeConfigValues(existing, value)
				continue
			}
			out[key] = value
		}
		return out
	}
	return right
}

// DigestSettings 为合并配置生成稳定摘要。
func DigestSettings(settings map[string]any) (string, error) {
	payload, err := json.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("digest runtime config: %w", err)
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
