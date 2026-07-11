package timezone

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
)

func TestInitConfigUsesDefaultTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, InitConfig(&config.Config{}))
		assertTimezone(t, DefaultTimezone)
	})
}

func TestInitConfigUsesConfiguredTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, InitConfig(&config.Config{System: config.SystemConfig{Timezone: "UTC"}}))
		assertTimezone(t, "UTC")
	})
}

func TestInitConfigReturnsErrorForInvalidTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		err := InitConfig(&config.Config{System: config.SystemConfig{Timezone: "Invalid/Timezone"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), `load timezone "Invalid/Timezone"`)
		require.NotEqual(t, "Invalid/Timezone", time.Local.String())
	})
}

func TestInitConfigOnlyInitializesOnceAfterSuccess(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, InitConfig(&config.Config{System: config.SystemConfig{Timezone: "UTC"}}))
		require.NoError(t, InitConfig(&config.Config{System: config.SystemConfig{Timezone: DefaultTimezone}}))
		assertTimezone(t, "UTC")
	})
}

func TestInitConfigCanRetryAfterFailure(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.Error(t, InitConfig(&config.Config{System: config.SystemConfig{Timezone: "Invalid/Timezone"}}))
		require.NoError(t, InitConfig(&config.Config{System: config.SystemConfig{Timezone: "UTC"}}))
		assertTimezone(t, "UTC")
	})
}

func assertTimezone(t *testing.T, want string) {
	t.Helper()
	require.Equal(t, want, time.Local.String())
	require.Equal(t, want, os.Getenv("TZ"))
}

func withIsolatedTimezone(t *testing.T, fn func()) {
	t.Helper()

	// timezone 初始化会修改进程级 TZ、time.Local 和包级 state；相关测试必须串行隔离，不能使用 t.Parallel。
	oldLocal := time.Local
	oldTZ, hadTZ := os.LookupEnv("TZ")
	if hadTZ {
		t.Setenv("TZ", oldTZ)
	} else {
		t.Setenv("TZ", "")
	}
	resetTimezoneStateForTest()
	t.Cleanup(func() {
		resetTimezoneStateForTest()
		time.Local = oldLocal
	})

	fn()
}

func resetTimezoneStateForTest() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.initialized = false
}
