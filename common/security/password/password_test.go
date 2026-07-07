package password

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	httpresponse "github.com/aegiscore/common/http/response"
)

func TestNewServiceRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "zero concurrency", opts: Options{Concurrency: 0, QueueSize: 1}},
		{name: "negative concurrency", opts: Options{Concurrency: -1, QueueSize: 1}},
		{name: "zero queue", opts: Options{Concurrency: 1, QueueSize: 0}},
		{name: "negative queue", opts: Options{Concurrency: 1, QueueSize: -1}},
		{name: "queue smaller than concurrency", opts: Options{Concurrency: 2, QueueSize: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(tt.opts)
			require.Error(t, err)
		})
	}
}

func TestHashContextAndVerifyContext(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)
	require.NotEqual(t, "secret", hash)
	require.True(t, strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$"))

	ok, err := service.VerifyContext(context.Background(), "secret", hash)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestVerifyContextRejectsWrongPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)
	ok, err := service.VerifyContext(context.Background(), "wrong", hash)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerifyContextRejectsMalformedHash(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	ok, err := service.VerifyContext(context.Background(), "secret", "not-a-hash")
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestVerifyContextRejectsOversizedHash(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	ok, err := service.VerifyContext(context.Background(), "secret", "$argon2id$v=19$m=65536,t=3,p=4$"+strings.Repeat("a", 513))
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestVerifyContextRejectsMalformedParams(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)

	malformed := strings.Replace(hash, "m=65536,t=3,p=4", "m=65536,t=3,p=4=bad", 1)
	ok, err := service.VerifyContext(context.Background(), "secret", malformed)
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestVerifyContextRejectsUnsupportedParams(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)

	unsupported := strings.Replace(hash, "m=65536,t=3,p=4", "m=32768,t=3,p=4", 1)
	ok, err := service.VerifyContext(context.Background(), "secret", unsupported)
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestVerifyContextRejectsUnsupportedSaltLength(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)
	parts := strings.Split(hash, "$")
	parts[4] = base64.RawStdEncoding.EncodeToString([]byte("short"))

	ok, err := service.VerifyContext(context.Background(), "secret", strings.Join(parts, "$"))
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestVerifyContextRejectsUnsupportedKeyLength(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)
	parts := strings.Split(hash, "$")
	parts[5] = base64.RawStdEncoding.EncodeToString([]byte("short"))

	ok, err := service.VerifyContext(context.Background(), "secret", strings.Join(parts, "$"))
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestHashContextRejectsEmptyPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	_, err := service.HashContext(context.Background(), "")
	require.ErrorIs(t, err, ErrEmptyPassword)
}

func TestVerifyContextRejectsEmptyPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)
	ok, err := service.VerifyContext(context.Background(), "", hash)
	require.ErrorIs(t, err, ErrEmptyPassword)
	require.False(t, ok)
}

func TestHashContextRejectsOversizedPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	_, err := service.HashContext(context.Background(), strings.Repeat("a", maxPasswordLength+1))
	require.ErrorIs(t, err, ErrPasswordTooLong)
}

func TestVerifyContextRejectsOversizedPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)
	ok, err := service.VerifyContext(context.Background(), strings.Repeat("a", maxPasswordLength+1), hash)
	require.ErrorIs(t, err, ErrPasswordTooLong)
	require.False(t, ok)
}

func TestHashContextReturnsBusyWhenQueueIsFull(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	service.queue <- struct{}{}

	_, err := service.HashContext(context.Background(), "secret")
	require.ErrorIs(t, err, ErrPasswordKDFBusy)
}

func TestErrPasswordKDFBusyIsRenderableApplicationError(t *testing.T) {
	wrapped := errors.Join(errors.New("outer"), ErrPasswordKDFBusy)
	require.ErrorIs(t, wrapped, ErrPasswordKDFBusy)

	var appErr *contracterrors.Error
	require.ErrorAs(t, ErrPasswordKDFBusy, &appErr)
	require.Equal(t, contracterrors.KindServiceUnavailable, appErr.Kind)
	require.Equal(t, reasonPasswordKDFBusy, appErr.Reason)
	require.Equal(t, contracterrors.CodeServiceUnavailable, appErr.Code)
	require.Equal(t, messagePasswordKDFBusy, appErr.Message)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/password", nil)

	httpresponse.Fail(ctx, ErrPasswordKDFBusy)

	var envelope contractresponse.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.False(t, envelope.Success)
	require.Equal(t, contracterrors.CodeServiceUnavailable, envelope.Code)
	require.Equal(t, messagePasswordKDFBusy, envelope.Message)
}

func TestHashContextCancelsWhileWaitingForKDFSlot(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	service.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := service.HashContext(ctx, "secret")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Empty(t, service.queue)
}

func TestServiceInstancesDoNotShareKDFControls(t *testing.T) {
	blocked := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	available := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	blocked.queue <- struct{}{}

	var wg sync.WaitGroup
	wg.Add(2)

	var blockedErr error
	go func() {
		defer wg.Done()
		_, blockedErr = blocked.HashContext(context.Background(), "secret")
	}()

	var availableErr error
	go func() {
		defer wg.Done()
		_, availableErr = available.HashContext(context.Background(), "secret")
	}()

	wg.Wait()
	require.ErrorIs(t, blockedErr, ErrPasswordKDFBusy)
	require.NoError(t, availableErr)
}

func newTestService(t *testing.T, opts Options) *Service {
	t.Helper()
	service, err := NewService(opts)
	require.NoError(t, err)
	return service
}
