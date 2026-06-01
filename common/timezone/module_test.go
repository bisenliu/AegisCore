package timezone

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
)

func TestInitConfigUsesDefaultTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		if err := InitConfig(&config.Config{}); err != nil {
			t.Fatalf("InitConfig: %v", err)
		}
		assertTimezone(t, DefaultTimezone)
	})
}

func TestInitConfigUsesConfiguredTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		if err := InitConfig(&config.Config{System: config.SystemConfig{Timezone: "UTC"}}); err != nil {
			t.Fatalf("InitConfig: %v", err)
		}
		assertTimezone(t, "UTC")
	})
}

func TestInitConfigReturnsErrorForInvalidTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		err := InitConfig(&config.Config{System: config.SystemConfig{Timezone: "Invalid/Timezone"}})
		if err == nil {
			t.Fatal("InitConfig error = nil")
		}
		if !strings.Contains(err.Error(), `load timezone "Invalid/Timezone"`) {
			t.Fatalf("InitConfig error = %q, want load timezone context", err.Error())
		}
		if time.Local.String() == "Invalid/Timezone" {
			t.Fatal("time.Local was set for invalid timezone")
		}
	})
}

func TestInitConfigOnlyInitializesOnceAfterSuccess(t *testing.T) {
	withIsolatedTimezone(t, func() {
		if err := InitConfig(&config.Config{System: config.SystemConfig{Timezone: "UTC"}}); err != nil {
			t.Fatalf("InitConfig first call: %v", err)
		}
		if err := InitConfig(&config.Config{System: config.SystemConfig{Timezone: DefaultTimezone}}); err != nil {
			t.Fatalf("InitConfig second call: %v", err)
		}
		assertTimezone(t, "UTC")
	})
}

func TestInitConfigCanRetryAfterFailure(t *testing.T) {
	withIsolatedTimezone(t, func() {
		if err := InitConfig(&config.Config{System: config.SystemConfig{Timezone: "Invalid/Timezone"}}); err == nil {
			t.Fatal("InitConfig invalid error = nil")
		}
		if err := InitConfig(&config.Config{System: config.SystemConfig{Timezone: "UTC"}}); err != nil {
			t.Fatalf("InitConfig retry: %v", err)
		}
		assertTimezone(t, "UTC")
	})
}

func assertTimezone(t *testing.T, want string) {
	t.Helper()
	if got := time.Local.String(); got != want {
		t.Fatalf("time.Local = %q, want %q", got, want)
	}
	if got := os.Getenv("TZ"); got != want {
		t.Fatalf("TZ = %q, want %q", got, want)
	}
}

func withIsolatedTimezone(t *testing.T, fn func()) {
	t.Helper()

	oldLocal := time.Local
	oldTZ, hadTZ := os.LookupEnv("TZ")
	state = timezoneState{}
	t.Cleanup(func() {
		state = timezoneState{}
		time.Local = oldLocal
		if hadTZ {
			_ = os.Setenv("TZ", oldTZ)
			return
		}
		_ = os.Unsetenv("TZ")
	})

	fn()
}
