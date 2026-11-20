package response

import contracterrors "github.com/aegiscore/platform/contract/errors"

// Envelope 是前后端约定的统一接口响应格式，所有 API 请求成功或失败均返回此结构。
// 包含是否成功标识（Success）、业务状态码（Code）、提示信息（Message）以及携带的具体业务数据（Data）或错误详情（Errors）。
type Envelope struct {
	Success bool                `json:"success"`
	Code    contracterrors.Code `json:"code"`
	Message string              `json:"message"`
	Data    any                 `json:"data,omitempty"`
	Errors  any                 `json:"errors,omitempty"`
}
