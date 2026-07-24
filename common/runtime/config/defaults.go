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
	// DefaultGinMode 是 Gin 的默认进程运行模式。
	DefaultGinMode = "release"
	// DefaultTimezone 是未声明进程时区时使用的稳定默认值。
	DefaultTimezone = "Asia/Shanghai"

	// DefaultMetricsPath 是 Prometheus metrics 的默认暴露路径。
	DefaultMetricsPath = "/metrics"
	// DefaultPprofAddr 是 pprof 诊断 listener 的本地安全默认地址。
	DefaultPprofAddr = "127.0.0.1:6060"
)

const (
	// defaultLifecycleStartTimeout 是配置加载后 Fx App.Start 和 OnStart hook 的启动总预算，不覆盖 fx.New 同步构造。
	defaultLifecycleStartTimeout = 60 * time.Second
	// defaultLifecycleStopTimeout 是 Fx app 关闭总预算，必须覆盖协议 drain、worker drain、tracing flush 和安全余量。
	defaultLifecycleStopTimeout = 120 * time.Second
	defaultHTTPReadTimeout      = 30 * time.Second
	defaultHTTPWriteTimeout     = 60 * time.Second
	defaultHTTPIdleTimeout      = 120 * time.Second
	defaultHTTPShutdownTimeout  = 10 * time.Second
	defaultGRPCShutdownTimeout  = 10 * time.Second
	defaultTracingSampleRatio   = 1.0

	// DefaultLifecycleWorkerDrainAllowance 是服务内 feature worker 在 App.Stop 总预算中的通用 drain 预留。
	DefaultLifecycleWorkerDrainAllowance = 30 * time.Second
	// DefaultLifecycleTracingFlushAllowance 是 tracing provider flush/shutdown 的通用预留。
	DefaultLifecycleTracingFlushAllowance = 5 * time.Second
	// DefaultLifecycleShutdownSafetyMargin 为 logger sync 和其他轻量 hook 保留余量。
	DefaultLifecycleShutdownSafetyMargin = 5 * time.Second
)

// DefaultConfig 返回可直接启动本地 HTTP 服务的核心配置。
func DefaultConfig() Config {
	return Config{
		App: AppConfig{
			Name:        DefaultAppName,
			Environment: DefaultAppEnvironment,
		},
		Runtime: RuntimeConfig{
			Lifecycle: LifecycleConfig{
				StartTimeout: defaultLifecycleStartTimeout,
				StopTimeout:  defaultLifecycleStopTimeout,
			},
			Gin: GinConfig{
				Mode: DefaultGinMode,
			},
			Timezone: DefaultTimezone,
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
			Pprof: PprofConfig{
				Enabled: false,
				Addr:    DefaultPprofAddr,
			},
		},
	}
}
