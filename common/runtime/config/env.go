package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	EnvService        = "AEGISCORE_SERVICE"
	EnvNacosAddr      = "AEGISCORE_NACOS_ADDR"
	EnvNacosNamespace = "AEGISCORE_NACOS_NAMESPACE"
	EnvNacosGroup     = "AEGISCORE_NACOS_GROUP"
	EnvNacosDataIDs   = "AEGISCORE_NACOS_DATA_IDS"
	EnvNacosTimeout   = "AEGISCORE_NACOS_TIMEOUT"
	EnvNacosUsername  = "AEGISCORE_NACOS_USERNAME"
	EnvNacosPassword  = "AEGISCORE_NACOS_PASSWORD"

	defaultNacosTimeout = 5 * time.Second
)

// NacosEnv 描述运行时通过环境变量选择出的 Nacos 配置来源。
type NacosEnv struct {
	Service   string
	Addr      string
	Namespace string
	Group     string
	DataIDs   []string
	Timeout   time.Duration
	Username  string
	Password  string
}

// SourceMetadata 描述一次成功配置加载的来源和摘要。
type SourceMetadata struct {
	Provider  string
	Service   string
	Namespace string
	Group     string
	DataIDs   []string
	Digest    string
}

// DataIDsCSV 返回稳定的 dataId 列表文本。
func (m SourceMetadata) DataIDsCSV() string {
	return strings.Join(m.DataIDs, ",")
}

// LoadNacosEnv 从当前进程环境变量读取 Nacos 来源配置。
func LoadNacosEnv() (NacosEnv, error) {
	return loadNacosEnv(os.LookupEnv)
}

func loadNacosEnv(lookup func(string) (string, bool)) (NacosEnv, error) {
	if lookup == nil {
		return NacosEnv{}, fmt.Errorf("read config env: lookup is required")
	}
	service, err := requiredEnv(lookup, EnvService)
	if err != nil {
		return NacosEnv{}, err
	}
	addr, err := requiredEnv(lookup, EnvNacosAddr)
	if err != nil {
		return NacosEnv{}, err
	}
	namespace, err := requiredEnv(lookup, EnvNacosNamespace)
	if err != nil {
		return NacosEnv{}, err
	}
	group, err := requiredEnv(lookup, EnvNacosGroup)
	if err != nil {
		return NacosEnv{}, err
	}
	timeout := defaultNacosTimeout
	if raw, ok := lookup(EnvNacosTimeout); ok && strings.TrimSpace(raw) != "" {
		parsed, parseErr := time.ParseDuration(strings.TrimSpace(raw))
		if parseErr != nil {
			return NacosEnv{}, fmt.Errorf("read config env: %s is invalid: %w", EnvNacosTimeout, parseErr)
		}
		if parsed <= 0 {
			return NacosEnv{}, fmt.Errorf("read config env: %s must be > 0", EnvNacosTimeout)
		}
		timeout = parsed
	}
	dataIDs, err := parseDataIDs(lookup, service)
	if err != nil {
		return NacosEnv{}, err
	}
	rawUsername, _ := lookup(EnvNacosUsername)
	username := strings.TrimSpace(rawUsername)
	password, passwordSet := lookup(EnvNacosPassword)
	passwordPresent := passwordSet && strings.TrimSpace(password) != ""
	if (username != "") != passwordPresent {
		return NacosEnv{}, fmt.Errorf(
			"read config env: %s and %s must be set together",
			EnvNacosUsername,
			EnvNacosPassword,
		)
	}
	return NacosEnv{
		Service: service, Addr: addr, Namespace: namespace, Group: group,
		DataIDs: dataIDs, Timeout: timeout, Username: username, Password: password,
	}, nil
}

func requiredEnv(lookup func(string) (string, bool), name string) (string, error) {
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("read config env: %s is required", name)
	}
	return strings.TrimSpace(value), nil
}

func parseDataIDs(lookup func(string) (string, bool), service string) ([]string, error) {
	raw, ok := lookup(EnvNacosDataIDs)
	if !ok || strings.TrimSpace(raw) == "" {
		return []string{"base.yaml", "resources.yaml", service + ".yaml"}, nil
	}
	parts := strings.Split(raw, ",")
	dataIDs := make([]string, 0, len(parts))
	for _, part := range parts {
		dataID := strings.TrimSpace(part)
		if dataID == "" {
			return nil, fmt.Errorf("read config env: %s contains empty dataId", EnvNacosDataIDs)
		}
		dataIDs = append(dataIDs, dataID)
	}
	return dataIDs, nil
}

func sourceMetadata(env NacosEnv, digest string) SourceMetadata {
	return SourceMetadata{
		Provider: "nacos", Service: env.Service, Namespace: env.Namespace,
		Group: env.Group, DataIDs: append([]string(nil), env.DataIDs...), Digest: digest,
	}
}
