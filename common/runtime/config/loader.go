package config

import (
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// envPrefix 定义环境变量配置覆盖使用的全局前缀。
const envPrefix = "AEGISCORE"

type defaultsApplier interface {
	ApplyDefaults()
}

// Load 从指定路径或默认 configs 目录读取 YAML 配置，并应用 AEGISCORE_ 环境变量覆盖。
func Load(path string) (*Config, error) {
	return LoadInto(path, Config.Validate)
}

// LoadInto 从配置文件加载调用方指定的配置结构，并应用 AEGISCORE_ 环境变量覆盖。
func LoadInto[T any](path string, validate func(T) error) (*T, error) {
	v := viper.New()
	setCoreDefaults(v, DefaultConfig())

	v.SetConfigType("yaml")
	if path != "" {
		v.SetConfigFile(path)
	} else {
		// 两个搜索路径分别支持从模块目录和服务子目录运行命令。
		v.SetConfigName("config")
		v.AddConfigPath("./configs")
		v.AddConfigPath("../configs")
	}

	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	for _, key := range v.AllKeys() {
		// 绑定已发现和已注册默认值的 key，使环境变量可以覆盖缺省配置。
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env %s: %w", key, err)
		}
	}

	var cfg T
	if err := validateKnownConfigKeys(cfg, v.AllSettings()); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if defaults, ok := any(&cfg).(defaultsApplier); ok {
		// 服务扩展配置可在校验前补齐自身默认值，并将结果保留在返回对象中。
		defaults.ApplyDefaults()
	}
	if validate != nil {
		if err := validate(cfg); err != nil {
			return nil, fmt.Errorf("validate config: %w", err)
		}
	}

	return &cfg, nil
}

func setCoreDefaults(v *viper.Viper, defaults Config) {
	v.SetDefault("app.name", defaults.App.Name)
	v.SetDefault("app.environment", defaults.App.Environment)
	v.SetDefault("runtime.lifecycle.start_timeout", defaults.Runtime.Lifecycle.StartTimeout)
	v.SetDefault("runtime.lifecycle.stop_timeout", defaults.Runtime.Lifecycle.StopTimeout)
	v.SetDefault("runtime.gin.mode", defaults.Runtime.Gin.Mode)
	v.SetDefault("server.http.enabled", defaults.Server.HTTP.Enabled)
	v.SetDefault("server.http.host", defaults.Server.HTTP.Host)
	v.SetDefault("server.http.port", defaults.Server.HTTP.Port)
	v.SetDefault("server.http.read_timeout", defaults.Server.HTTP.ReadTimeout)
	v.SetDefault("server.http.write_timeout", defaults.Server.HTTP.WriteTimeout)
	v.SetDefault("server.http.idle_timeout", defaults.Server.HTTP.IdleTimeout)
	v.SetDefault("server.http.shutdown_timeout", defaults.Server.HTTP.ShutdownTimeout)
	v.SetDefault("server.grpc.enabled", defaults.Server.GRPC.Enabled)
	v.SetDefault("server.grpc.host", defaults.Server.GRPC.Host)
	v.SetDefault("server.grpc.port", defaults.Server.GRPC.Port)
	v.SetDefault("server.grpc.shutdown_timeout", defaults.Server.GRPC.ShutdownTimeout)
	v.SetDefault("log.level", defaults.Log.Level)
	v.SetDefault("log.format", defaults.Log.Format)
	v.SetDefault("observability.metrics.enabled", defaults.Observability.Metrics.Enabled)
	v.SetDefault("observability.metrics.path", defaults.Observability.Metrics.Path)
	v.SetDefault("observability.metrics.include_runtime", defaults.Observability.Metrics.IncludeRuntime)
	v.SetDefault("observability.tracing.enabled", defaults.Observability.Tracing.Enabled)
	v.SetDefault("observability.tracing.sample_ratio", defaults.Observability.Tracing.SampleRatio)
	v.SetDefault("observability.tracing.otlp_endpoint", defaults.Observability.Tracing.OTLPEndpoint)
	v.SetDefault("observability.tracing.insecure", defaults.Observability.Tracing.Insecure)
	v.SetDefault("observability.pprof.enabled", defaults.Observability.Pprof.Enabled)
	v.SetDefault("observability.pprof.addr", defaults.Observability.Pprof.Addr)
}
