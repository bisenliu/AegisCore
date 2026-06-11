package response

import contracterrors "github.com/aegiscore/common/contract/errors"

const (
	// MessageOK 是查询类 API 响应的默认成功消息。
	MessageOK = "ok"
	// MessageCreated 是创建类 API 响应的默认成功消息。
	MessageCreated = "created"
	// MessageInternalError 是对外暴露的非敏感服务器错误消息。
	MessageInternalError = contracterrors.MessageInternalError
	// MessageAuthInvalid 是登录状态无效或过期时展示给用户的消息。
	MessageAuthInvalid = "登录状态无效或已过期，请重新登录"
)
