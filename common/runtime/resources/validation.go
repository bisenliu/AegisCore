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
		if strings.TrimSpace(cfg.Addr) == "" {
			errs = append(errs, fieldError(resourcePath+".addr", "is required"))
		} else {
			errs = append(errs, validateHostPort(resourcePath+".addr", cfg.Addr)...)
		}
		if cfg.DB < 0 {
			errs = append(errs, fieldError(resourcePath+".db", "must be >= 0"))
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
