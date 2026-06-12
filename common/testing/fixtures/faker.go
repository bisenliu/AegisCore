package fixtures

import (
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

type Faker struct {
	base string
	seq  atomic.Uint64
}

func NewFaker(t testing.TB) *Faker {
	t.Helper()
	base := slug(t.Name())
	if base == "" {
		base = "test"
	}
	return &Faker{base: base + "-" + uuid.NewString()[:8]}
}

func (f *Faker) UniqueSuffix() string {
	next := f.seq.Add(1)
	return f.base + "-" + strconv.FormatUint(next, 10)
}

func (f *Faker) Username(prefix string) string {
	return joinSlug(prefix, f.UniqueSuffix())
}

func (f *Faker) Email(prefix string) string {
	return joinSlug(prefix, f.UniqueSuffix()) + "@example.test"
}

func (f *Faker) Name(prefix string) string {
	cleanPrefix := strings.TrimSpace(prefix)
	if cleanPrefix == "" {
		cleanPrefix = "Test"
	}
	return cleanPrefix + " " + f.UniqueSuffix()
}

func (f *Faker) UUIDString() string {
	return uuid.NewString()
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
