package rediskey

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// Separator 是 Redis key 分段的统一分隔符。
	Separator = ":"
)

var (
	// ErrInvalidSegment 表示 Redis key 分段不符合统一构造规则。
	ErrInvalidSegment = errors.New("invalid redis key segment")
)

// Options 包含 Redis key builder 的通用配置。
type Options struct {
	Namespace string
}

// Builder 构造带可选 namespace 和 scope 的 Redis key。
type Builder struct {
	namespace string
	scope     []string
}

// NewBuilder 构造无业务语义的 Redis key builder。
func NewBuilder(options Options) (Builder, error) {
	namespace := strings.TrimSpace(options.Namespace)
	if namespace != "" {
		if err := validateSegment(namespace); err != nil {
			return Builder{}, fmt.Errorf("redis key namespace: %w", err)
		}
	}
	return Builder{namespace: namespace}, nil
}

// MustBuilder 构造 Redis key builder，配置无效时 panic。
func MustBuilder(options Options) Builder {
	builder, err := NewBuilder(options)
	if err != nil {
		panic(err)
	}
	return builder
}

// Scoped 返回带额外固定 scope 的新 builder。
func (b Builder) Scoped(parts ...string) (Builder, error) {
	scope := make([]string, 0, len(b.scope)+len(parts))
	for _, part := range b.scope {
		if err := validateSegment(part); err != nil {
			return Builder{}, fmt.Errorf("redis key scope: %w", err)
		}
		scope = append(scope, part)
	}
	for _, part := range parts {
		if err := validateSegment(part); err != nil {
			return Builder{}, fmt.Errorf("redis key scope: %w", err)
		}
		scope = append(scope, part)
	}
	return Builder{namespace: b.namespace, scope: scope}, nil
}

// MustScoped 返回带额外固定 scope 的新 builder，scope 无效时 panic。
func (b Builder) MustScoped(parts ...string) Builder {
	builder, err := b.Scoped(parts...)
	if err != nil {
		panic(err)
	}
	return builder
}

// Key 使用 namespace、scope 和动态分段构造完整 Redis key。
func (b Builder) Key(parts ...string) (string, error) {
	all := make([]string, 0, 1+len(b.scope)+len(parts))
	if b.namespace != "" {
		if err := validateSegment(b.namespace); err != nil {
			return "", fmt.Errorf("redis key namespace: %w", err)
		}
		all = append(all, b.namespace)
	}
	for _, part := range b.scope {
		if err := validateSegment(part); err != nil {
			return "", fmt.Errorf("redis key scope: %w", err)
		}
		all = append(all, part)
	}
	for _, part := range parts {
		if err := validateSegment(part); err != nil {
			return "", fmt.Errorf("redis key part: %w", err)
		}
		all = append(all, part)
	}
	if len(all) == 0 {
		return "", fmt.Errorf("%w: empty key", ErrInvalidSegment)
	}
	return strings.Join(all, Separator), nil
}

// MustKey 构造完整 Redis key，分段无效时 panic。
func (b Builder) MustKey(parts ...string) string {
	key, err := b.Key(parts...)
	if err != nil {
		panic(err)
	}
	return key
}

// Prefix 使用 namespace、scope 和动态分段构造带尾部分隔符的 Redis key 前缀。
func (b Builder) Prefix(parts ...string) (string, error) {
	key, err := b.Key(parts...)
	if err != nil {
		return "", err
	}
	return key + Separator, nil
}

// MustPrefix 构造 Redis key 前缀，分段无效时 panic。
func (b Builder) MustPrefix(parts ...string) string {
	prefix, err := b.Prefix(parts...)
	if err != nil {
		panic(err)
	}
	return prefix
}

// HashTag 返回 Redis Cluster hash tag 分段。
func HashTag(value string) string {
	return "{" + value + "}"
}

func validateSegment(segment string) error {
	if segment == "" {
		return fmt.Errorf("%w: empty segment", ErrInvalidSegment)
	}
	isHashTag, err := validateHashTag(segment)
	if err != nil {
		return err
	}
	if !isHashTag && strings.Contains(segment, Separator) {
		return fmt.Errorf("%w: segment contains separator %q", ErrInvalidSegment, Separator)
	}
	return nil
}

func validateHashTag(segment string) (bool, error) {
	openCount := strings.Count(segment, "{")
	closeCount := strings.Count(segment, "}")
	if openCount == 0 && closeCount == 0 {
		return false, nil
	}
	if openCount != 1 || closeCount != 1 || !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
		return false, fmt.Errorf("%w: malformed hash tag", ErrInvalidSegment)
	}
	if len(segment) == len("{}") {
		return false, fmt.Errorf("%w: empty hash tag", ErrInvalidSegment)
	}
	return true, nil
}
