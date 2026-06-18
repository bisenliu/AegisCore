package middleware

import (
	"context"
	"errors"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	spanAttrErrorType  = "error.type"
	spanAttrHTTPStatus = "http_status"

	spanErrorTypePanic = "panic"
)

var errPanicRecovered = errors.New("panic recovered")

// recordPanicOnSpan 将 panic 以脱敏形式记录到当前 span。
func recordPanicOnSpan(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() || !span.IsRecording() {
		return
	}
	span.RecordError(errPanicRecovered, trace.WithAttributes(attribute.String(spanAttrErrorType, spanErrorTypePanic)))
	span.SetStatus(codes.Error, errPanicRecovered.Error())
}

// markServerErrorStatus 将 5xx HTTP 响应标记为 span error。
func markServerErrorStatus(ctx context.Context, status int) {
	if status < http.StatusInternalServerError {
		return
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() || !span.IsRecording() {
		return
	}
	span.SetAttributes(attribute.Int(spanAttrHTTPStatus, status))
	span.SetStatus(codes.Error, "http server error")
}
