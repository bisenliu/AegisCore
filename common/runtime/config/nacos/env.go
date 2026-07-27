package nacos

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	EnvService   = "AEGISCORE_SERVICE"
	EnvAddr      = "AEGISCORE_NACOS_ADDR"
	EnvNamespace = "AEGISCORE_NACOS_NAMESPACE"
	EnvGroup     = "AEGISCORE_NACOS_GROUP"
	EnvDataIDs   = "AEGISCORE_NACOS_DATA_IDS"
	EnvTimeout   = "AEGISCORE_NACOS_TIMEOUT"
	EnvUsername  = "AEGISCORE_NACOS_USERNAME"
	EnvPassword  = "AEGISCORE_NACOS_PASSWORD" // #nosec G101 -- 常量仅表示环境变量名称，不包含凭据。

	defaultTimeout = 5 * time.Second
)

// Env 描述运行时通过环境变量选择出的 Nacos 配置来源。
type Env struct {
	Service   string
	Addr      string
	Namespace string
	Group     string
	DataIDs   []string
	Timeout   time.Duration
	Username  string
	Password  string
}

// LoadEnv 从当前进程环境变量读取 Nacos 来源配置。
func LoadEnv() (Env, error) {
	return loadEnv(os.LookupEnv)
}

func loadEnv(lookup func(string) (string, bool)) (Env, error) {
	if lookup == nil {
		return Env{}, fmt.Errorf("read config env: lookup is required")
	}
	service, err := requiredEnv(lookup, EnvService)
	if err != nil {
		return Env{}, err
	}
	addr, err := requiredEnv(lookup, EnvAddr)
	if err != nil {
		return Env{}, err
	}
	namespace, err := requiredEnv(lookup, EnvNamespace)
	if err != nil {
		return Env{}, err
	}
	group, err := requiredEnv(lookup, EnvGroup)
	if err != nil {
		return Env{}, err
	}
	timeout := defaultTimeout
	if raw, ok := lookup(EnvTimeout); ok && strings.TrimSpace(raw) != "" {
		parsed, parseErr := time.ParseDuration(strings.TrimSpace(raw))
		if parseErr != nil {
			return Env{}, fmt.Errorf("read config env: %s is invalid: %w", EnvTimeout, parseErr)
		}
		if parsed <= 0 {
			return Env{}, fmt.Errorf("read config env: %s must be > 0", EnvTimeout)
		}
		timeout = parsed
	}
	dataIDs, err := parseDataIDs(lookup, service)
	if err != nil {
		return Env{}, err
	}
	rawUsername, _ := lookup(EnvUsername)
	username := strings.TrimSpace(rawUsername)
	password, passwordSet := lookup(EnvPassword)
	passwordPresent := passwordSet && strings.TrimSpace(password) != ""
	if (username != "") != passwordPresent {
		return Env{}, fmt.Errorf(
			"read config env: %s and %s must be set together",
			EnvUsername,
			EnvPassword,
		)
	}
	return Env{
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
	raw, ok := lookup(EnvDataIDs)
	if !ok || strings.TrimSpace(raw) == "" {
		return []string{"base.yaml", "resources.yaml", service + ".yaml"}, nil
	}
	parts := strings.Split(raw, ",")
	dataIDs := make([]string, 0, len(parts))
	for _, part := range parts {
		dataID := strings.TrimSpace(part)
		if dataID == "" {
			return nil, fmt.Errorf("read config env: %s contains empty dataId", EnvDataIDs)
		}
		dataIDs = append(dataIDs, dataID)
	}
	return dataIDs, nil
}
