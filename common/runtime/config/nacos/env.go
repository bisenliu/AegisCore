package nacos

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// EnvService 环境变量名，用来指定当前要启动的服务名称（例如 user-service），系统据此加载对应配置。
	EnvService = "AEGISCORE_SERVICE"
	// EnvAddr 环境变量名，用来指定远程配置中心的连接地址（格式为 主机名:端口）。
	EnvAddr = "AEGISCORE_NACOS_ADDR"
	// EnvNamespace 环境变量名，用来指定远程配置中心的环境分区标识（用于隔离开发、测试或生产环境）。
	EnvNamespace = "AEGISCORE_NACOS_NAMESPACE"
	// EnvGroup 环境变量名，用来指定远程配置中心的分组名称，用于归类同一套配置项。
	EnvGroup = "AEGISCORE_NACOS_GROUP"
	// EnvDataIDs 环境变量名，用来指定服务启动时需要从配置中心下载哪些配置文件（多个文件用逗号隔开）。
	EnvDataIDs = "AEGISCORE_NACOS_DATA_IDS"
	// EnvTimeout 环境变量名，用来指定连接远程配置中心的最长等待超时时间。
	EnvTimeout = "AEGISCORE_NACOS_TIMEOUT"
	// EnvUsername 环境变量名，用来指定登录远程配置中心的管理账号。
	EnvUsername = "AEGISCORE_NACOS_USERNAME"
	// EnvPassword 环境变量名，用来指定登录远程配置中心的管理密码（仅记录环境变量名称，不包含真实凭据）。
	EnvPassword = "AEGISCORE_NACOS_PASSWORD" // #nosec G101 -- 常量仅表示环境变量名称，不包含凭据。

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

// loadEnv 从操作系统环境变量中读取远程配置中心的连接地址、环境标识和账号密码；如果关键连接信息未配置则报错终止启动。
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

// requiredEnv 读取指定的必填环境变量；如果环境变量未设置或内容全为空格则返回错误提示。
func requiredEnv(lookup func(string) (string, bool), name string) (string, error) {
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("read config env: %s is required", name)
	}
	return strings.TrimSpace(value), nil
}

// parseDataIDs 将逗号分隔的配置文件名列表拆分成独立的名称清单；未指定时默认使用当前服务的基础配置。
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
