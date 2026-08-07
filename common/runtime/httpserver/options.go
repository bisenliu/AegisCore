package httpserver

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// Options 包含 Managed HTTP server 的完整构造参数。
//
// Options 不填充默认值。Name、Addr、Handler 必填，ShutdownTimeout 必须
// 大于零，其他 timeout 不得为负数。
type Options struct {
	Name            string
	Addr            string
	Handler         http.Handler
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	OnServeError    func(error)
}

func validateOptions(options Options) error {
	name := strings.TrimSpace(options.Name)
	addr := strings.TrimSpace(options.Addr)
	invalid := func(field, reason string) error {
		return fmt.Errorf(
			"validate http server %q at %q options: %w: %s %s",
			name,
			addr,
			ErrInvalidOptions,
			field,
			reason,
		)
	}

	if name == "" {
		return invalid("name", "is required")
	}
	if addr == "" {
		return invalid("addr", "is required")
	}
	if isNilHandler(options.Handler) {
		return invalid("handler", "is required")
	}
	if options.ReadTimeout < 0 {
		return invalid("read timeout", "must not be negative")
	}
	if options.WriteTimeout < 0 {
		return invalid("write timeout", "must not be negative")
	}
	if options.IdleTimeout < 0 {
		return invalid("idle timeout", "must not be negative")
	}
	if options.ShutdownTimeout <= 0 {
		return invalid("shutdown timeout", "must be positive")
	}
	return nil
}

func isNilHandler(handler http.Handler) bool {
	if handler == nil {
		return true
	}
	value := reflect.ValueOf(handler)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
