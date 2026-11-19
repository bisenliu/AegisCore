package response

import (
	"context"
	"errors"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	contracterrors "github.com/aegiscore/common/contract/errors"
)

const (
	// spanAttrErrorCode 是链路追踪中记录业务错误码的属性键名。
	spanAttrErrorCode = "error_code"
	// spanAttrErrorType 是链路追踪中记录错误分类类型的属性键名。
	spanAttrErrorType = "error.type"
	// spanAttrHTTPStatus 是链路追踪中记录 HTTP 响应状态码的属性键名。
	spanAttrHTTPStatus = "http_status"

	// spanErrorTypeApplication 表示业务应用层主动返回的错误类型标记。
	spanErrorTypeApplication = "application_error"
)

var errApplicationError = errors.New("application error")

// annotateAppErrorOnSpan 将应用错误的低基数字段写入当前 span。
func annotateAppErrorOnSpan(ctx context.Context, err *contracterrors.Error) {
	if err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() || !span.IsRecording() {
		return
	}
	status := statusCode(err)
	attrs := []attribute.KeyValue{
		attribute.Int(spanAttrErrorCode, int(err.Code)),
		attribute.Int(spanAttrHTTPStatus, status),
	}
	span.SetAttributes(attrs...)
	if status < http.StatusInternalServerError {
		return
	}
	span.SetStatus(codes.Error, errApplicationError.Error())
	if err.Cause != nil {
		span.RecordError(errApplicationError, trace.WithAttributes(append(attrs, attribute.String(spanAttrErrorType, spanErrorTypeApplication))...))
	}
}
