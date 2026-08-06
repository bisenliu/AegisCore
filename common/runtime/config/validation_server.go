package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const (
	minPort = 1
	maxPort = 65535
)

// validateServer 校验 HTTP/gRPC server 是否至少启用一个，并检查各自地址和超时字段。
func (c Config) validateServer() []error {
	var errs []error
	if !c.Server.HTTP.Enabled && !c.Server.GRPC.Enabled {
		errs = append(errs, FieldError("server", "must enable at least one of server.http or server.grpc"))
	}
	if c.Server.HTTP.Enabled {
		errs = append(errs, validateServerAddress("server.http", c.Server.HTTP.Host, c.Server.HTTP.Port)...)
		errs = append(errs, ValidatePositiveDuration("server.http.read_timeout", c.Server.HTTP.ReadTimeout)...)
		errs = append(errs, ValidatePositiveDuration("server.http.write_timeout", c.Server.HTTP.WriteTimeout)...)
		errs = append(errs, ValidatePositiveDuration("server.http.idle_timeout", c.Server.HTTP.IdleTimeout)...)
		errs = append(errs, ValidatePositiveDuration("server.http.shutdown_timeout", c.Server.HTTP.ShutdownTimeout)...)
		errs = append(errs, validateTrustedProxies(c.Server.HTTP.TrustedProxies)...)
	}
	if c.Server.GRPC.Enabled {
		errs = append(errs, validateServerAddress("server.grpc", c.Server.GRPC.Host, c.Server.GRPC.Port)...)
		errs = append(errs, ValidatePositiveDuration("server.grpc.shutdown_timeout", c.Server.GRPC.ShutdownTimeout)...)
	}
	return errs
}

// validateServerAddress 校验 server host 非空且 port 位于 TCP 端口范围内。
func validateServerAddress(base string, host string, port int) []error {
	var errs []error
	if strings.TrimSpace(host) == "" {
		errs = append(errs, FieldError(base+".host", "is required"))
	}
	return append(errs, validatePort(base+".port", port)...)
}

// validateTrustedProxies 校验 Gin trusted proxy 只接受 IP 或 CIDR，并保留原索引到错误路径。
func validateTrustedProxies(values []string) []error {
	var errs []error
	for index, value := range values {
		path := fmt.Sprintf("server.http.trusted_proxies[%d]", index)
		proxy := strings.TrimSpace(value)
		if proxy == "" {
			errs = append(errs, FieldError(path, "must be an IP address or CIDR"))
			continue
		}
		if net.ParseIP(proxy) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(proxy); err == nil {
			continue
		}
		errs = append(errs, FieldError(path, "must be an IP address or CIDR"))
	}
	return errs
}

// validatePort 校验数值型端口字段。
func validatePort(path string, value int) []error {
	if value < minPort || value > maxPort {
		return []error{FieldError(path, fmt.Sprintf("must be between %d and %d", minPort, maxPort))}
	}
	return nil
}

// validatePortText 校验 host:port 拆分后的文本端口字段。
func validatePortText(path string, value string) []error {
	port, err := strconv.Atoi(value)
	if err != nil || port < minPort || port > maxPort {
		return []error{FieldError(path, fmt.Sprintf("port must be between %d and %d", minPort, maxPort))}
	}
	return nil
}
