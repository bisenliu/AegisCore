package timezone

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInitUsesDefaultTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, os.Unsetenv("TZ"))
		require.NoError(t, Init())
		assertTimezone(t, DefaultTimezone)
	})
}

func TestInitUsesPlatformTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, os.Setenv("TZ", "UTC"))
		require.NoError(t, Init())
		assertTimezone(t, "UTC")
	})
}

func TestInitReturnsErrorForInvalidTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, os.Setenv("TZ", "Invalid/Timezone"))
		err := Init()
		require.Error(t, err)
		require.Contains(t, err.Error(), `load timezone "Invalid/Timezone"`)
		require.NotEqual(t, "Invalid/Timezone", time.Local.String())
	})
}

func TestInitOnlyInitializesOnceAfterSuccess(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, os.Setenv("TZ", "UTC"))
		require.NoError(t, Init())
		require.NoError(t, os.Setenv("TZ", DefaultTimezone))
		require.NoError(t, Init())
		require.Equal(t, "UTC", time.Local.String())
		require.Equal(t, DefaultTimezone, os.Getenv("TZ"))
	})
}

func TestInitCanRetryAfterFailure(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, os.Setenv("TZ", "Invalid/Timezone"))
		require.Error(t, Init())
		require.NoError(t, os.Setenv("TZ", "UTC"))
		require.NoError(t, Init())
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
