package config

import "time"

const (
	// DefaultAppName 是未声明服务名称时使用的本地开发标识。
	DefaultAppName = "aegiscore"
	// DefaultAppEnvironment 是未声明部署环境时使用的本地环境标识。
	DefaultAppEnvironment = "local"

	// DefaultHTTPHost 是 HTTP server 的本地安全监听地址。
	DefaultHTTPHost = "127.0.0.1"
	// DefaultHTTPPort 是 HTTP server 的默认监听端口。
	DefaultHTTPPort = 8080
	// DefaultGRPCHost 是 gRPC server 的本地安全监听地址。
	DefaultGRPCHost = "127.0.0.1"
	// DefaultGRPCPort 是 gRPC server 的默认监听端口。
	DefaultGRPCPort = 9090

	// DefaultMetricsPath 是 Prometheus metrics 的默认暴露路径。
	DefaultMetricsPath = "/metrics"
)

const (
	defaultHTTPReadTimeout     = 30 * time.Second
	defaultHTTPWriteTimeout    = 60 * time.Second
	defaultHTTPIdleTimeout     = 120 * time.Second
	defaultHTTPShutdownTimeout = 10 * time.Second
	defaultGRPCShutdownTimeout = 10 * time.Second
	defaultTracingSampleRatio  = 1.0
)

// DefaultConfig 返回可直接启动本地 HTTP 服务的核心配置。
func DefaultConfig() Config {
	return Config{
		App: AppConfig{
			Name:        DefaultAppName,
			Environment: DefaultAppEnvironment,
		},
		Server: ServerConfig{
			HTTP: HTTPServerConfig{
				Enabled:         true,
				Host:            DefaultHTTPHost,
				Port:            DefaultHTTPPort,
				ReadTimeout:     defaultHTTPReadTimeout,
				WriteTimeout:    defaultHTTPWriteTimeout,
				IdleTimeout:     defaultHTTPIdleTimeout,
				ShutdownTimeout: defaultHTTPShutdownTimeout,
			},
			GRPC: GRPCServerConfig{
				Enabled:         false,
				Host:            DefaultGRPCHost,
				Port:            DefaultGRPCPort,
				ShutdownTimeout: defaultGRPCShutdownTimeout,
			},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Observability: ObservabilityConfig{
			Metrics: MetricsConfig{
				Enabled:        false,
				Path:           DefaultMetricsPath,
				IncludeRuntime: true,
			},
			Tracing: TracingConfig{
				Enabled:     false,
				SampleRatio: defaultTracingSampleRatio,
				Insecure:    false,
			},
		},
	}
}
