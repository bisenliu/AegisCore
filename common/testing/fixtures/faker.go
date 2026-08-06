package fixtures

import (
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	runtimeid "github.com/aegiscore/common/runtime/id"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

// Faker 为单个测试生成可追踪且并发安全的唯一标识和常用字段值。
type Faker struct {
	base string
	seq  atomic.Uint64
}

// NewFaker 创建以测试名和随机 UUID 片段为前缀的 Faker。
func NewFaker(t testing.TB) *Faker {
	t.Helper()
	base := slug(t.Name())
	if base == "" {
		base = "test"
	}
	return &Faker{base: base + "-" + newUUIDString()[:8]}
}

// UniqueSuffix 返回当前 Faker 内唯一的 slug；该方法可被多个 goroutine 并发调用。
func (f *Faker) UniqueSuffix() string {
	next := f.seq.Add(1)
	return f.base + "-" + strconv.FormatUint(next, 10)
}

// Username 返回带调用方前缀的唯一用户名。
func (f *Faker) Username(prefix string) string {
	return joinSlug(prefix, f.UniqueSuffix())
}

// Email 返回 example.test 域下带调用方前缀的唯一邮箱地址。
func (f *Faker) Email(prefix string) string {
	return joinSlug(prefix, f.UniqueSuffix()) + "@example.test"
}

// Name 返回带调用方前缀的唯一显示名称；空前缀使用 Test。
func (f *Faker) Name(prefix string) string {
	cleanPrefix := strings.TrimSpace(prefix)
	if cleanPrefix == "" {
		cleanPrefix = "Test"
	}
	return cleanPrefix + " " + f.UniqueSuffix()
}

// UUIDString 返回一个新的 UUID 字符串。
func (f *Faker) UUIDString() string {
	return newUUIDString()
}

func newUUIDString() string {
	return runtimeid.MustNewUUIDString()
}

func joinSlug(prefix, suffix string) string {
	cleanPrefix := slug(prefix)
	if cleanPrefix == "" {
		cleanPrefix = "test"
	}
	return cleanPrefix + "-" + suffix
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonSlugChars.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	return value
}
