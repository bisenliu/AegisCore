package response

import (
	"encoding/json"
	"testing"

	contracterrors "github.com/aegiscore/common/contract/errors"
)

func TestEnvelopeJSONShape(t *testing.T) {
	envelope := Envelope{
		Success: true,
		Code:    contracterrors.CodeOK,
		Message: MessageOK,
		Data:    map[string]any{"id": 1},
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if got["success"] != true || got["code"] != float64(contracterrors.CodeOK) || got["message"] != MessageOK {
		t.Fatalf("envelope = %#v", got)
	}
	if _, ok := got["errors"]; ok {
		t.Fatalf("errors = %#v, want omitted", got["errors"])
	}
}

func TestFailureEnvelopeJSONShape(t *testing.T) {
	envelope := Envelope{
		Success: false,
		Code:    contracterrors.CodeValidationFailed,
		Message: "请求参数验证失败",
		Errors:  []map[string]string{{"field": "email", "message": "邮箱格式不正确"}},
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if got["success"] != false || got["code"] != float64(contracterrors.CodeValidationFailed) || got["message"] != "请求参数验证失败" {
		t.Fatalf("envelope = %#v", got)
	}
	if _, ok := got["data"]; ok {
		t.Fatalf("data = %#v, want omitted", got["data"])
	}
	errors, ok := got["errors"].([]any)
	if !ok || len(errors) != 1 {
		t.Fatalf("errors = %#v, want one error", got["errors"])
	}
}

func TestMessages(t *testing.T) {
	if MessageOK != "ok" || MessageCreated != "created" || MessageInternalError != contracterrors.MessageInternalError || MessageAuthInvalid == "" {
		t.Fatalf("messages = (%q,%q,%q,%q)", MessageOK, MessageCreated, MessageInternalError, MessageAuthInvalid)
	}
}
