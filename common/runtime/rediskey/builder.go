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
	// Namespace 表示服务或环境级统一命名空间前缀，可选。
	Namespace string
	// Scope 表示固定的业务模块分段作用域，可选。
	Scope []string
}

// Builder 构造带可选 namespace 和 scope 的 Redis key。
type Builder struct {
	namespace string
	scope     []string
	prefix    string
}

// NewBuilder 创建通用的 Redis 键名生成器（负责统一命名空间前缀与冒号分隔符规则，本身不绑定特定业务逻辑）。
func NewBuilder(options Options) (Builder, error) {
	namespace := strings.TrimSpace(options.Namespace)
	if namespace != "" {
		if err := validateSegment(namespace); err != nil {
			return Builder{}, fmt.Errorf("redis key namespace: %w", err)
		}
	}

	scope := make([]string, 0, len(options.Scope))
	for _, part := range options.Scope {
		if err := validateSegment(part); err != nil {
			return Builder{}, fmt.Errorf("redis key scope: %w", err)
		}
		scope = append(scope, part)
	}

	var prefixBuilder strings.Builder
	if namespace != "" {
		prefixBuilder.WriteString(namespace)
		for _, part := range scope {
			prefixBuilder.WriteString(Separator)
			prefixBuilder.WriteString(part)
		}
	} else {
		for i, part := range scope {
			if i > 0 {
				prefixBuilder.WriteString(Separator)
			}
			prefixBuilder.WriteString(part)
		}
	}

	return Builder{
		namespace: namespace,
		scope:     scope,
		prefix:    prefixBuilder.String(),
	}, nil
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
	if len(parts) == 0 {
		return b, nil
	}
	for _, part := range parts {
		if err := validateSegment(part); err != nil {
			return Builder{}, fmt.Errorf("redis key scope: %w", err)
		}
	}

	scope := make([]string, len(b.scope), len(b.scope)+len(parts))
	copy(scope, b.scope)
	scope = append(scope, parts...)

	var prefixBuilder strings.Builder
	if b.prefix != "" {
		prefixBuilder.WriteString(b.prefix)
		for _, part := range parts {
			prefixBuilder.WriteString(Separator)
			prefixBuilder.WriteString(part)
		}
	} else {
		for i, part := range parts {
			if i > 0 {
				prefixBuilder.WriteString(Separator)
			}
			prefixBuilder.WriteString(part)
		}
	}

	return Builder{
		namespace: b.namespace,
		scope:     scope,
		prefix:    prefixBuilder.String(),
	}, nil
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
	if len(parts) == 0 {
		if b.prefix != "" {
			return b.prefix, nil
		}
		if b.namespace != "" || len(b.scope) > 0 {
			// 防御直接使用 Builder{} 字面量初始化且未走 NewBuilder/Scoped 的极端场景
			return b.buildFallbackKey(nil)
		}
		return "", fmt.Errorf("%w: empty key", ErrInvalidSegment)
	}

	for _, part := range parts {
		if err := validateSegment(part); err != nil {
			return "", fmt.Errorf("redis key part: %w", err)
		}
	}

	base := b.prefix
	if base == "" && (b.namespace != "" || len(b.scope) > 0) {
		return b.buildFallbackKey(parts)
	}

	var totalLen int
	if base != "" {
		totalLen = len(base) + len(Separator)
	}
	for i, part := range parts {
		if i > 0 {
			totalLen += len(Separator)
		}
		totalLen += len(part)
	}

	var sb strings.Builder
	sb.Grow(totalLen)
	if base != "" {
		sb.WriteString(base)
		sb.WriteString(Separator)
	}
	for i, part := range parts {
		if i > 0 {
			sb.WriteString(Separator)
		}
		sb.WriteString(part)
	}
	return sb.String(), nil
}

// buildFallbackKey 针对字面量构造未缓存 prefix 的情况进行兜底构造。
func (b Builder) buildFallbackKey(parts []string) (string, error) {
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
	all = append(all, parts...)
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
	if len(parts) == 0 && b.prefix != "" {
		return b.prefix + Separator, nil
	}
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

// HashTag 用花括号将分段包裹（形如 "{user_123}"），用于满足 Redis Cluster 分布式分片规范。
//
// 为什么需要 HashTag（原理与业务背景）：
//  1. 默认路由机制：Redis Cluster 默认对整个 key 字符串计算 CRC16 哈希值并取模（CRC16(key) % 16384），将不同的 key
//     自动分散到集群的 16384 个哈希槽（Hash Slot）上。单 key 的 GET/SET 不需要任何手动指定。
//  2. 集群铁律约束：Redis Cluster 严禁跨节点的多 key 操作。所有在同一批次执行的多 key 指令（如 MGET/MSET、MULTI/EXEC 事务、
//     以及在同一个 Lua 脚本中同时读写的多个 key），必须位于完全相同的哈希槽上，否则 Redis 会直接报
//     "CROSSSLOT Keys in request don't hash to the same slot" 严重错误并拒绝执行。
//  3. HashTag 规约：Redis 官方规范规定，若 key 中包含成对的花括号 "{...}"，计算哈希槽时将“仅提取花括号内部的子串”进行哈希计算。
//     因此，对同属一个业务聚合根（如同一用户、同一订单）的多个关联 key 使用相同的 HashTag（如 HashTag(userID)），
//     能确保它们百分之百落入同一个物理分片节点，从而允许在 Lua 脚本或事务中安全地进行多 key 原子操作。
//
// 使用建议：
//   - 无需使用：全局独立、单 key 读写的普通缓存或分布式锁（由 Redis 自动负载均衡打散至不同节点即可）。
//   - 必须使用：需要在同一 Lua 脚本、事务或批量命令中同时操作的多个关联 key（通常包裹业务主体 ID，如 userID、orderID）。
func HashTag(value string) string {
	return "{" + value + "}"
}

// validateSegment 检查键名片段是否为空或夹带冒号，避免破坏缓存键的层级命名规则。
func validateSegment(segment string) error {
	if segment == "" {
		return fmt.Errorf("%w: empty segment", ErrInvalidSegment)
	}

	var openBrace, closeBrace, colonCount int
	for i := 0; i < len(segment); i++ {
		switch segment[i] {
		case '{':
			openBrace++
		case '}':
			closeBrace++
		case ':':
			colonCount++
		}
	}

	if openBrace == 0 && closeBrace == 0 {
		if colonCount > 0 {
			return fmt.Errorf("%w: segment contains separator %q", ErrInvalidSegment, Separator)
		}
		return nil
	}

	if openBrace != 1 || closeBrace != 1 || segment[0] != '{' || segment[len(segment)-1] != '}' {
		return fmt.Errorf("%w: malformed hash tag", ErrInvalidSegment)
	}
	if len(segment) == 2 {
		return fmt.Errorf("%w: empty hash tag", ErrInvalidSegment)
	}
	return nil
}
