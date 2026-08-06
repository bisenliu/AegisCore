package tracing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/aegiscore/common/runtime/config"
)

const defaultOTLPTimeout = 5 * time.Second

// newOTLPExporter 基于 tracing 配置创建 OTLP gRPC span exporter。
//
// 该函数只在启用 tracing 的启动路径中调用；disabled provider 不会要求 endpoint，也不会创建
// exporter。返回错误统一通过 wrapOTLPExporterError 保留底层 cause，便于启动失败定位。
func newOTLPExporter(ctx context.Context, cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	endpoint := trimSpace(cfg.OTLPEndpoint)
	if endpoint == "" {
		return nil, errors.New("otlp tracing endpoint is required")
	}
	clientOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTimeout(defaultOTLPTimeout),
	}
	if cfg.Insecure {
		clientOptions = append(clientOptions, otlptracegrpc.WithInsecure())
	}
	traceExporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(clientOptions...))
	if err != nil {
		return nil, wrapOTLPExporterError(err)
	}
	return traceExporter, nil
}

// wrapOTLPExporterError 为 OTLP exporter 构造失败补充稳定错误上下文。
//
// 调用方可以通过 errors.Is 或 errors.As 继续识别底层 cause，同时日志和测试能稳定匹配
// `create OTLP tracing exporter` 前缀。
func wrapOTLPExporterError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("create OTLP tracing exporter: %w", err)
}
