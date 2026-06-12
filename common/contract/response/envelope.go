package response

import contracterrors "github.com/aegiscore/common/contract/errors"

// Envelope 是成功和失败 API 响应共用的 HTTP 响应信封。
type Envelope struct {
	Success bool                `json:"success"`
	Code    contracterrors.Code `json:"code"`
	Message string              `json:"message"`
	Data    any                 `json:"data,omitempty"`
	Errors  any                 `json:"errors,omitempty"`
}
