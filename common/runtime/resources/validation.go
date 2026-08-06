package resources

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

const (
	minPort = 1
	maxPort = 65535
)

// Validate 校验全部具名 Redis 资源，并在错误中保留调用方配置路径。
func (c RedisConfigs) Validate(basePath string) error {
	var errs []error
	for _, name := range sortedNames(c) {
		cfg := c[name]
		resourcePath := namedResourcePath(basePath, name)
		if strings.TrimSpace(name) == "" {
			errs = append(errs, fieldError(basePath, "must not contain an empty named resource"))
		}
		mode := strings.TrimSpace(cfg.Mode)
		switch mode {
		case RedisModeStandalone:
			if strings.TrimSpace(cfg.Addr) == "" {
				errs = append(errs, fieldError(resourcePath+".addr", "is required when mode is standalone"))
			} else {
				errs = append(errs, validateHostPort(resourcePath+".addr", cfg.Addr)...)
			}
			if len(cfg.Addrs) > 0 {
				errs = append(errs, fieldError(resourcePath+".addrs", "must be empty when mode is standalone"))
			}
			if cfg.Cluster.MaxRedirects != 0 {
				errs = append(errs, fieldError(resourcePath+".cluster.max_redirects", "must be 0 when mode is standalone"))
			}
		case RedisModeCluster:
			if strings.TrimSpace(cfg.Addr) != "" {
				errs = append(errs, fieldError(resourcePath+".addr", "must be empty when mode is cluster"))
			}
			if len(cfg.Addrs) == 0 {
				errs = append(errs, fieldError(resourcePath+".addrs", "must contain at least one address"))
			}
			for idx, addr := range cfg.Addrs {
				addrPath := fmt.Sprintf("%s.addrs[%d]", resourcePath, idx)
				if strings.TrimSpace(addr) == "" {
					errs = append(errs, fieldError(addrPath, "is required"))
					continue
				}
				errs = append(errs, validateHostPort(addrPath, addr)...)
			}
			if cfg.Cluster.MaxRedirects < 0 {
				errs = append(errs, fieldError(resourcePath+".cluster.max_redirects", "must be >= 0"))
			}
		default:
			errs = append(errs, fieldError(resourcePath+".mode", "must be standalone or cluster"))
		}
		errs = append(errs, validatePositiveDuration(resourcePath+".timeout", cfg.Timeout)...)
	}
	return errors.Join(errs...)
}

// Validate 校验全部具名 PostgreSQL 资源，并在错误中保留调用方配置路径。
func (c PostgresConfigs) Validate(basePath string) error {
	var errs []error
	for _, name := range sortedNames(c) {
		cfg := c[name]
		resourcePath := namedResourcePath(basePath, name)
		if strings.TrimSpace(name) == "" {
			errs = append(errs, fieldError(basePath, "must not contain an empty named resource"))
		}
		if strings.TrimSpace(cfg.Host) == "" {
			errs = append(errs, fieldError(resourcePath+".host", "is required"))
		}
		errs = append(errs, validatePort(resourcePath+".port", cfg.Port)...)
		if strings.TrimSpace(cfg.Username) == "" {
			errs = append(errs, fieldError(resourcePath+".username", "is required"))
		}
		if strings.TrimSpace(cfg.DBName) == "" {
			errs = append(errs, fieldError(resourcePath+".db_name", "is required"))
		}

		sslMode := strings.ToLower(strings.TrimSpace(cfg.SSLMode))
		if sslMode == "" {
			errs = append(errs, fieldError(resourcePath+".sslmode", "is required"))
		} else if !isValidPostgresSSLMode(sslMode) {
			errs = append(errs, fieldError(resourcePath+".sslmode", "must be a valid PostgreSQL sslmode"))
		}

		poolPath := resourcePath + ".pool"
		if cfg.Pool.MaxOpenConns <= 0 {
			errs = append(errs, fieldError(poolPath+".max_open_conns", "must be > 0"))
		}
		if cfg.Pool.MaxIdleConns < 0 {
			errs = append(errs, fieldError(poolPath+".max_idle_conns", "must be >= 0"))
		}
		if cfg.Pool.MaxOpenConns > 0 && cfg.Pool.MaxIdleConns > cfg.Pool.MaxOpenConns {
			errs = append(errs, fieldError(poolPath+".max_idle_conns", "must be <= max_open_conns"))
		}
		errs = append(errs, validatePositiveDuration(poolPath+".conn_max_lifetime", cfg.Pool.ConnMaxLifetime)...)
		errs = append(errs, validatePositiveDuration(poolPath+".conn_max_idle_time", cfg.Pool.ConnMaxIdleTime)...)
	}
	return errors.Join(errs...)
}

func sortedNames[T any](values map[string]T) []string {
	// 配置校验错误会直接用于诊断输出，排序可避免 map 遍历顺序造成结果抖动。
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func namedResourcePath(basePath string, name string) string {
	if name == "" {
		return basePath
	}
	return basePath + "." + name
}

func validateHostPort(path string, value string) []error {
	host, portText, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" {
		return []error{fieldError(path, "must be in host:port format")}
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		return []error{fieldError(path, "port must be valid")}
	}
	return validatePort(path, port)
}

func validatePort(path string, value int) []error {
	if value < minPort || value > maxPort {
		return []error{fieldError(path, fmt.Sprintf("must be between %d and %d", minPort, maxPort))}
	}
	return nil
}

func validatePositiveDuration(path string, value time.Duration) []error {
	if value <= 0 {
		return []error{fieldError(path, "must be > 0")}
	}
	return nil
}

func isValidPostgresSSLMode(value string) bool {
	switch value {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return true
	default:
		return false
	}
}

func fieldError(path string, message string) error {
	return errors.New(path + " " + message)
}
