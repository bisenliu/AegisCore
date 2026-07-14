package logger

import (
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsIgnorableSyncError(t *testing.T) {
	for _, err := range []error{syscall.EBADF, syscall.EINVAL, syscall.ENOTTY} {
		require.True(t, isIgnorableSyncError(fmt.Errorf("sync logger: %w", err)))
	}
	for _, err := range []error{nil, errors.New("write failed")} {
		require.False(t, isIgnorableSyncError(err))
	}
}
