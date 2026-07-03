package response

import (
	"encoding/json"
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"

	"github.com/stretchr/testify/require"
)

func TestEnvelopeJSONShape(t *testing.T) {
	envelope := Envelope{
		Success: true,
		Code:    contracterrors.CodeOK,
		Message: MessageOK,
		Data:    map[string]any{"id": 1},
	}

	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, true, got["success"])
	require.Equal(t, float64(contracterrors.CodeOK), got["code"])
	require.Equal(t, MessageOK, got["message"])
	require.NotContains(t, got, "errors")
}

func TestFailureEnvelopeJSONShape(t *testing.T) {
	envelope := Envelope{
		Success: false,
		Code:    contracterrors.CodeValidationFailed,
		Message: "请求参数验证失败",
		Errors:  []map[string]string{{"field": "email", "message": "邮箱格式不正确"}},
	}

	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, false, got["success"])
	require.Equal(t, float64(contracterrors.CodeValidationFailed), got["code"])
	require.Equal(t, "请求参数验证失败", got["message"])
	require.NotContains(t, got, "data")
	errors, ok := got["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errors, 1)
}

func TestMessages(t *testing.T) {
	require.Equal(t, "ok", MessageOK)
	require.Equal(t, "created", MessageCreated)
	require.Equal(t, contracterrors.MessageInternalError, MessageInternalError)
	require.NotEmpty(t, MessageAuthInvalid)
}
