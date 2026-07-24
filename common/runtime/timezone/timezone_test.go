package timezone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInitUsesDefaultTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, Init("Asia/Shanghai"))
		assertTimezone(t, "Asia/Shanghai")
	})
}

func TestInitUsesConfiguredTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, Init("UTC"))
		assertTimezone(t, "UTC")
	})
}

func TestInitReturnsErrorForInvalidTimezone(t *testing.T) {
	withIsolatedTimezone(t, func() {
		err := Init("Invalid/Timezone")
		require.Error(t, err)
		require.Contains(t, err.Error(), `load timezone "Invalid/Timezone"`)
		require.NotEqual(t, "Invalid/Timezone", time.Local.String())
	})
}

func TestInitOnlyInitializesOnceAfterSuccess(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.NoError(t, Init("UTC"))
		require.NoError(t, Init("Asia/Shanghai"))
		require.Equal(t, "UTC", time.Local.String())
	})
}

func TestInitCanRetryAfterFailure(t *testing.T) {
	withIsolatedTimezone(t, func() {
		require.Error(t, Init("Invalid/Timezone"))
		require.NoError(t, Init("UTC"))
		assertTimezone(t, "UTC")
	})
}

func assertTimezone(t *testing.T, want string) {
	t.Helper()
	require.Equal(t, want, time.Local.String())
}

func withIsolatedTimezone(t *testing.T, fn func()) {
	t.Helper()

	// timezone 初始化会修改进程级 time.Local 和包级 state；相关测试必须串行隔离，不能使用 t.Parallel。
	oldLocal := time.Local
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
