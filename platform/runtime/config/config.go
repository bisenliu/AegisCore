package config

import "time"

// Config 是跨服务共享的最小 runtime 配置对象。
type Config struct {
	App           AppConfig           `mapstructure:"app"`
	Runtime       RuntimeConfig       `mapstructure:"runtime"`
	Server        ServerConfig        `mapstructure:"server"`
	Log           LogConfig           `mapstructure:"log"`
	Observability ObservabilityConfig `mapstructure:"observability"`
}

// AppConfig 标识运行中的服务和部署环境。
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

// RuntimeConfig 包含进程级 runtime 生命周期配置。
// 该配置约束 Fx App.Start/App.Stop 生命周期阶段，不覆盖配置加载或 fx.New 同步构造，也不替代 HTTP、gRPC 等组件级关闭超时。
type RuntimeConfig struct {
	Lifecycle LifecycleConfig `mapstructure:"lifecycle"`
	Gin       GinConfig       `mapstructure:"gin"`
	Timezone  string          `mapstructure:"timezone"`
}

// GinConfig 控制 Gin 包级进程运行模式。
type GinConfig struct {
	Mode string `mapstructure:"mode"`
}

// LifecycleConfig 包含 Fx App.Start 和 App.Stop 的总预算。
// StopTimeout 必须覆盖已启用或已配置协议 server 的 shutdown timeout 和共享 runtime 已知串行停止预算。
type LifecycleConfig struct {
	StartTimeout time.Duration `mapstructure:"start_timeout"`
	StopTimeout  time.Duration `mapstructure:"stop_timeout"`
}

// ServerConfig 包含共享协议 server 的生命周期配置。
type ServerConfig struct {
	HTTP HTTPServerConfig `mapstructure:"http"`
	GRPC GRPCServerConfig `mapstructure:"grpc"`
}

// HTTPServerConfig 包含 HTTP server 的监听、超时、关闭和可信代理设置。
type HTTPServerConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies"`
}

// GRPCServerConfig 包含 gRPC server 的最小监听和关闭设置。
type GRPCServerConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// LogConfig 控制 logger 级别和编码格式。
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// ObservabilityConfig 包含可观测性运行时配置入口。
type ObservabilityConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing"`
	Pprof   PprofConfig   `mapstructure:"pprof"`
}

// PprofConfig 控制独立 pprof 诊断监听。
type PprofConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Addr    string `mapstructure:"addr"`
}

// MetricsConfig 包含 metrics 采集和暴露的配置契约。
type MetricsConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	Path           string `mapstructure:"path"`
	IncludeRuntime bool   `mapstructure:"include_runtime"`
}

// TracingConfig 包含 OTLP tracing 采样和传输配置。
type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	SampleRatio  float64 `mapstructure:"sample_ratio"`
	OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
	Insecure     bool    `mapstructure:"insecure"`
}
