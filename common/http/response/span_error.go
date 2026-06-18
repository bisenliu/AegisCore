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
	spanAttrErrorCode  = "error_code"
	spanAttrErrorType  = "error.type"
	spanAttrHTTPStatus = "http_status"

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
