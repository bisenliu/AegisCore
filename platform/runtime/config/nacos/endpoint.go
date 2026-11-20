package nacos

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// serverURLs 解析逗号分隔的 Nacos server 地址列表，并保持声明顺序用于 failover。
func serverURLs(addr string) ([]*url.URL, error) {
	parts := strings.Split(addr, ",")
	servers := make([]*url.URL, 0, len(parts))
	for _, part := range parts {
		server, err := serverURL(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, nil
}

// serverURL 解析单个 Nacos server 地址。
// 地址必须包含 host 和 port；未显式声明 scheme 时默认使用 http，未声明 context path 时默认使用 /nacos。
func serverURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("nacos server address is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse nacos server address %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("parse nacos server address %q: scheme must be http or https", raw)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("parse nacos server address %q: credentials, query and fragment are not allowed", raw)
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("parse nacos server address %q: host is required", raw)
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil || port == 0 {
		return nil, fmt.Errorf("parse nacos server address %q: port is required", raw)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = defaultContextPath
	}
	parsed.RawPath = ""
	return parsed, nil
}

// endpoint 基于 server context path 构造具体 Nacos API URL，并清空 query/fragment 避免跨请求串扰。
func endpoint(server *url.URL, apiPath string) *url.URL {
	result := *server
	result.Path = path.Join(server.Path, apiPath)
	result.RawPath = ""
	result.RawQuery = ""
	result.Fragment = ""
	return &result
}

// serverOrigin 返回带 context path 的 server 标识，用于 failover 错误中定位失败节点。
func serverOrigin(server *url.URL) string {
	return server.Scheme + "://" + server.Host + server.Path
}
