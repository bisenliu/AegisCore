package main

// TemplateFile 描述一个要生成的目标文件及其模板内容。
type TemplateFile struct {
	// Path 目标文件相对于仓库根目录的路径模板。
	Path string
	// Content 文件的模板内容。
	Content string
}

// scaffoldTemplates 定义新服务所需生成的全部骨架文件和部署模板列表。
var scaffoldTemplates = []TemplateFile{
	// 1. 服务命令行入口
	{
		Path: "{{.ServiceName}}/cmd/main.go",
		Content: `package main

// @title AegisCore {{.TitleName}} API
// @version 1.0.0
// @description AegisCore {{.TitleName}} API 服务，提供核心业务能力与健康探针。
// @host localhost:{{.Port}}
// @BasePath /api/v1
// @schemes http https
// @accept json
// @produce json

import (
	"fmt"
	"os"
)

// main 是 {{.TitleName}} 进程的总入口，负责解析命令行指令并启动服务。
func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`,
	},
	// 2. serve 子命令
	{
		Path: "{{.ServiceName}}/cmd/serve.go",
		Content: `package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"

	platformlogger "github.com/aegiscore/platform/runtime/logger"
	"github.com/aegiscore/{{.ServiceName}}/internal/bootstrap"
	serviceconfig "github.com/aegiscore/{{.ServiceName}}/internal/config"
)

// newRootCommand 创建 {{.TitleName}} 的根命令。
func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "{{.CommandName}}",
		Short: "{{.TitleName}} 服务进程入口",
	}
	rootCmd.AddCommand(newServeCommand())
	return rootCmd
}

// newServeCommand 创建启动 HTTP 服务的 serve 子命令。
func newServeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 {{.TitleName}} HTTP 运行时",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context())
		},
	}
	return cmd
}

// runServe 执行服务启动流程，加载配置、构造 Fx 运行时并监听优雅退出信号。
func runServe(ctx context.Context) error {
	loaded, err := serviceconfig.Load(ctx)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg := loaded.Config
	lifecycleCfg := cfg.Runtime.Lifecycle

	upstreamCtx := ctx
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := bootstrap.NewApp(cfg)

	startCtx, cancelStart := context.WithTimeout(ctx, lifecycleCfg.StartTimeout)
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		return fmt.Errorf("start app: %w", err)
	}

	var shutdownSignal fx.ShutdownSignal
	select {
	case <-ctx.Done():
	case shutdownSignal = <-app.Wait():
		platformlogger.NamedComponent(zap.L(), "server", "lifecycle").Info("received shutdown signal", zap.String("signal", shutdownSignal.String()))
	}

	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(upstreamCtx), lifecycleCfg.StopTimeout)
	defer cancelStop()
	stopErr := app.Stop(stopCtx)

	var exitCodeErr error
	if shutdownSignal.ExitCode != 0 {
		exitCodeErr = fmt.Errorf("application shutdown requested with exit code %d", shutdownSignal.ExitCode)
	}
	if stopErr != nil {
		stopErr = fmt.Errorf("stop application: %w", stopErr)
	}
	return errors.Join(exitCodeErr, stopErr)
}
`,
	},
	// 3. 配置加载与结构定义
	{
		Path: "{{.ServiceName}}/internal/config/config.go",
		Content: `package config

import (
	"context"

	platformconfig "github.com/aegiscore/platform/runtime/config"
	platformnacos "github.com/aegiscore/platform/runtime/config/nacos"
	platformresources "github.com/aegiscore/platform/runtime/resources"
)

// Config 是 {{.ServiceName}} 的根配置结构体。
type Config struct {
	platformconfig.Config ` + "`" + `mapstructure:",squash"` + "`" + `
	Resources             ResourcesConfig ` + "`" + `mapstructure:"resources"` + "`" + `
	HTTP                  HTTPConfig      ` + "`" + `mapstructure:"http"` + "`" + `
}

// ResourcesConfig 包含 {{.ServiceName}} 声明或从环境继承的外部基础设施资源（如 Redis、PostgreSQL）。
type ResourcesConfig struct {
	Redis    platformresources.RedisConfigs    ` + "`" + `mapstructure:"redis"` + "`" + `
	Postgres platformresources.PostgresConfigs ` + "`" + `mapstructure:"postgres"` + "`" + `
}

// HTTPConfig 包含 {{.ServiceName}} 私有 HTTP 配置。
type HTTPConfig struct {
	RequestBodyMaxBytes int64 ` + "`" + `mapstructure:"request_body_max_bytes"` + "`" + `
}

// sensitiveConfigPaths 声明需要脱敏打印的配置路径列表。
var sensitiveConfigPaths = []string{
	"resources.redis.*.password",
	"resources.postgres.*.password",
}

// SensitiveConfigPaths 返回 {{.ServiceName}} 拥有的敏感配置路径副本。
func SensitiveConfigPaths() []string {
	return append([]string(nil), sensitiveConfigPaths...)
}

// RedactEffectiveSettings 使用敏感路径策略返回脱敏配置副本。
func RedactEffectiveSettings(settings map[string]any) map[string]any {
	return platformconfig.RedactSettings(settings, sensitiveConfigPaths)
}

// LoadResult 包含加载完成的配置对象与来源元数据。
type LoadResult struct {
	Config *Config
	Source platformconfig.SourceMetadata
}

// Load 从 Nacos 分层配置或环境变量加载 {{.ServiceName}} 配置。
func Load(ctx context.Context) (*LoadResult, error) {
	env, err := platformnacos.LoadEnv()
	if err != nil {
		return nil, err
	}
	source, err := platformnacos.NewSource(env)
	if err != nil {
		return nil, err
	}
	settings, metadata, err := platformconfig.LoadSource(ctx, source)
	if err != nil {
		return nil, err
	}
	return DecodeSettings(settings, metadata)
}

// DecodeSettings 将原始配置 map 解码为 Config 结构。
func DecodeSettings(settings map[string]any, source platformconfig.SourceMetadata) (*LoadResult, error) {
	cfg, err := platformconfig.DecodeStrict(settings, platformconfig.DecodeOptions[Config]{
		Defaults: DefaultConfig,
		Validate: Config.Validate,
	})
	if err != nil {
		return nil, err
	}
	return &LoadResult{Config: cfg, Source: source}, nil
}

// DefaultConfig 返回 {{.ServiceName}} 的默认配置初值。
func DefaultConfig() Config {
	cfg := Config{
		Config: platformconfig.DefaultConfig(),
		HTTP: HTTPConfig{
			RequestBodyMaxBytes: 64 * 1024,
		},
	}
	cfg.App.Name = "aegiscore-{{.ServiceName}}"
	cfg.Server.HTTP.Port = {{.Port}}
	return cfg
}

// Validate 在服务启动前校验配置的合法性。
func (c Config) Validate() error {
	var errs []error
	if err := c.Config.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Resources.Redis.Validate("resources.redis"); err != nil {
		errs = append(errs, err)
	}
	if err := c.Resources.Postgres.Validate("resources.postgres"); err != nil {
		errs = append(errs, err)
	}
	if c.HTTP.RequestBodyMaxBytes <= 0 {
		errs = append(errs, platformconfig.FieldError("http.request_body_max_bytes", "must be > 0"))
	}
	if len(errs) == 0 {
		return nil
	}
	return platformconfig.NewValidationError(errs)
}

// RuntimeConfig 返回共享 runtime 配置副本。
func (c Config) RuntimeConfig() platformconfig.Config {
	return c.Config
}

// NewRuntimeConfig 提供共享 runtime 配置供 Fx 提供。
func NewRuntimeConfig(cfg *Config) *platformconfig.Config {
	runtime := cfg.RuntimeConfig()
	return &runtime
}

// NewHTTPSettings 从服务配置中提取 HTTP 专有设置供 Fx 提供。
func NewHTTPSettings(cfg *Config) HTTPConfig {
	return cfg.HTTP
}

// NewResourceSettings 从服务配置中提取外部基础设施资源设置供 Fx 提供。
func NewResourceSettings(cfg *Config) ResourcesConfig {
	return cfg.Resources
}

// TimezoneSettings 承载时区配置信息。
type TimezoneSettings struct {
	Name string
}

// NewTimezoneSettings 提取时区名称。
func NewTimezoneSettings(cfg *Config) TimezoneSettings {
	return TimezoneSettings{Name: cfg.Runtime.Timezone}
}
`,
	},
	// 4. Fx bootstrap app
	{
		Path: "{{.ServiceName}}/internal/bootstrap/app.go",
		Content: `package bootstrap

import (
	"go.uber.org/fx"

	"github.com/aegiscore/platform/runtime/logger"
	platformtz "github.com/aegiscore/platform/runtime/timezone"
	"github.com/aegiscore/platform/validation"
	serviceconfig "github.com/aegiscore/{{.ServiceName}}/internal/config"
	"github.com/aegiscore/{{.ServiceName}}/internal/providers"
)

// WiringModule 汇总声明 {{.TitleName}} 所需的所有组件依赖关系（仅声明，不立即执行启动网络或后台任务）。
var WiringModule = fx.Module("aegiscore-{{.ServiceName}}-wiring",
	// 横切能力：输入校验能力。
	validation.Module,
	// 基础基础设施与传输 provider 装配。
	providers.WiringModule,
	fx.Provide(
		// 对外 HTTP server 及其生命周期 hook。
		NewHTTPServer,
	),
)

// RuntimeModule 注册服务在启动运行时需要主动执行的初始化操作与生命周期。
var RuntimeModule = fx.Module("aegiscore-{{.ServiceName}}-runtime",
	// 基础运行时：进程时区初始化。
	fx.Invoke(InitProcessRuntime),
	// 服务级 runtime 注册。
	providers.RuntimeModule,
	// 强制解析运行时 server 并注册启停 hook。
	fx.Invoke(registerRuntimeServers),
)

// registerRuntimeServers 触发 HTTP 服务的初始化，使其随系统生命周期一同启停。
func registerRuntimeServers(_ *HTTPRuntime) {}

// InitProcessRuntime 初始化 {{.TitleName}} 拥有的进程级 runtime 状态。
func InitProcessRuntime(settings serviceconfig.TimezoneSettings) error {
	return platformtz.Init(settings.Name)
}

// AppModule 汇总 {{.TitleName}} 的全部组件装配与运行时启动逻辑。
var AppModule = fx.Module("aegiscore-{{.ServiceName}}",
	WiringModule,
	RuntimeModule,
)

// AppOptions 从已解析的 service config 构建无配置 I/O 的基础 Fx options。
func AppOptions(cfg *serviceconfig.Config, additional ...fx.Option) []fx.Option {
	lifecycleCfg := cfg.Runtime.Lifecycle
	options := []fx.Option{
		fx.RecoverFromPanics(),
		fx.Supply(cfg),
		fx.Provide(
			serviceconfig.NewRuntimeConfig,
			serviceconfig.NewHTTPSettings,
			serviceconfig.NewResourceSettings,
			serviceconfig.NewTimezoneSettings,
			logger.NewLogger,
		),
		fx.WithLogger(logger.NewFxEventLogger),
		fx.StartTimeout(lifecycleCfg.StartTimeout),
		fx.StopTimeout(lifecycleCfg.StopTimeout),
	}
	return append(options, additional...)
}

// NewApp 使用调用方已解析的配置构建 {{.TitleName}} Fx 应用。
func NewApp(cfg *serviceconfig.Config) *fx.App {
	return fx.New(AppOptions(
		cfg,
		AppModule,
	)...)
}
`,
	},
	// 5. HTTP server 管理
	{
		Path: "{{.ServiceName}}/internal/bootstrap/server.go",
		Content: `package bootstrap

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/config"
	"github.com/aegiscore/platform/runtime/httpserver"
	"github.com/aegiscore/platform/runtime/logger"
)

// HTTPRuntime 表达业务 HTTP server 的启用状态和可选运行实例。
type HTTPRuntime struct {
	Enabled bool
	Managed *httpserver.Managed
}

// HTTPServerParams 包含注册业务 HTTP server 生命周期所需的 Fx 输入。
type HTTPServerParams struct {
	fx.In

	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Config     *config.Config
	Log        *zap.Logger
	Engine     *gin.Engine
}

// NewHTTPServer 根据已解析配置创建业务 HTTP runtime，并在启用时注册 Fx hook。
func NewHTTPServer(params HTTPServerParams) (*HTTPRuntime, error) {
	if params.Config == nil {
		return nil, fmt.Errorf("create http runtime: config is required")
	}
	httpCfg := params.Config.Server.HTTP
	runtime := &HTTPRuntime{Enabled: httpCfg.Enabled}
	if !runtime.Enabled {
		return runtime, nil
	}
	if params.Lifecycle == nil {
		return nil, fmt.Errorf("create http runtime: lifecycle is required")
	}
	if params.Engine == nil {
		return nil, fmt.Errorf("create http runtime: gin engine is required")
	}

	addr := net.JoinHostPort(httpCfg.Host, strconv.Itoa(httpCfg.Port))
	httpLog := logger.NamedComponent(params.Log, "http", "http-server")
	managed, err := httpserver.New(httpserver.Options{
		Name:            "http",
		Addr:            addr,
		Handler:         params.Engine,
		ReadTimeout:     httpCfg.ReadTimeout,
		WriteTimeout:    httpCfg.WriteTimeout,
		IdleTimeout:     httpCfg.IdleTimeout,
		ShutdownTimeout: httpCfg.ShutdownTimeout,
		OnServeError: func(serveErr error) {
			httpLog.Error("http server failed unexpectedly", zap.Error(serveErr))
			_ = params.Shutdowner.Shutdown()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create http runtime: %w", err)
	}
	runtime.Managed = managed

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.WithContext(ctx, httpLog).Info("starting http server",
				zap.String("addr", addr),
				zap.String("service", params.Config.App.Name),
				zap.String("environment", params.Config.App.Environment),
				zap.String("timezone", time.Local.String()),
			)
			return managed.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			logger.WithContext(ctx, httpLog).Info("stopping http server")
			return managed.Stop(ctx)
		},
	})
	return runtime, nil
}
`,
	},
	// 6. providers 汇总
	{
		Path: "{{.ServiceName}}/internal/providers/fx.go",
		Content: `package providers

import (
	"go.uber.org/fx"

	providerobservability "github.com/aegiscore/{{.ServiceName}}/internal/providers/observability"
	providertransport "github.com/aegiscore/{{.ServiceName}}/internal/providers/transport"
)

// WiringModule 汇总声明 {{.TitleName}} 底层的基础设施组件。
var WiringModule = fx.Module("{{.ServiceName}}-providers-wiring",
	providerobservability.WiringModule,
	providertransport.WiringModule,
)

// RuntimeModule 注册底层在系统启动时需要立即激活的运行时任务。
var RuntimeModule = fx.Module("{{.ServiceName}}-providers-runtime",
	providerobservability.RuntimeModule,
	providertransport.RuntimeModule,
)
`,
	},
	// 7. 可观测性 provider
	{
		Path: "{{.ServiceName}}/internal/providers/observability/fx.go",
		Content: `package observability

import (
	"go.uber.org/fx"
)

// WiringModule 声明可观测性相关组件。
var WiringModule = fx.Module("{{.ServiceName}}-observability-wiring")

// RuntimeModule 注册启动期可观测性任务。
var RuntimeModule = fx.Module("{{.ServiceName}}-observability-runtime")
`,
	},
	// 8. 传输层 provider
	{
		Path: "{{.ServiceName}}/internal/providers/transport/fx.go",
		Content: `package transport

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/config"
	"github.com/aegiscore/{{.ServiceName}}/internal/router"
)

// WiringModule 声明 HTTP 传输层组件。
var WiringModule = fx.Module("{{.ServiceName}}-transport-wiring",
	fx.Provide(
		NewGinEngine,
	),
)

// RuntimeModule 声明 HTTP 运行时注册。
var RuntimeModule = fx.Module("{{.ServiceName}}-transport-runtime")

// GinEngineParams 包含构造 Gin 引擎的依赖项。
type GinEngineParams struct {
	fx.In

	Config *config.Config
	Log    *zap.Logger
}

// NewGinEngine 创建并初始化预置好路由与中间件的 Gin 引擎。
func NewGinEngine(params GinEngineParams) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	router.RegisterRoutes(engine, params.Config)
	return engine
}
`,
	},
	// 9. 路由注册与 /api/v1 示例
	{
		Path: "{{.ServiceName}}/internal/router/router.go",
		Content: `package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/aegiscore/platform/runtime/config"
)

// RegisterRoutes 注册 {{.TitleName}} 的所有公开路由与健康探针。
func RegisterRoutes(engine *gin.Engine, cfg *config.Config) {
	RegisterHealthRoutes(engine)

	v1 := engine.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"service": cfg.App.Name,
				"message": "pong",
			})
		})
	}
}
`,
	},
	// 10. 健康检查探针
	{
		Path: "{{.ServiceName}}/internal/router/health.go",
		Content: `package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterHealthRoutes 挂载健康检查探针路由（/livez, /readyz, /startupz）。
func RegisterHealthRoutes(engine *gin.Engine) {
	engine.GET("/livez", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	engine.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	engine.GET("/startupz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
`,
	},
	// 10.1 健康检查单元测试
	{
		Path: "{{.ServiceName}}/internal/router/health_test.go",
		Content: `package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterHealthRoutes(engine)

	endpoints := []string{"/livez", "/readyz", "/startupz"}
	for _, endpoint := range endpoints {
		w := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			t.Fatalf("create request: %v", err)
		}
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d for %s, got %d", http.StatusOK, endpoint, w.Code)
		}
	}
}
`,
	},
	// 11. go.mod
	{
		Path: "{{.ServiceName}}/go.mod",
		Content: `module github.com/aegiscore/{{.ServiceName}}

go 1.27.4

require (
	github.com/aegiscore/platform v0.0.0
	github.com/gin-gonic/gin v1.11.0
	github.com/spf13/cobra v1.10.1
	go.uber.org/fx v1.24.0
	go.uber.org/zap v1.27.1
)

replace github.com/aegiscore/platform => ../platform
`,
	},
	// 12. 服务 Makefile
	{
		Path: "{{.ServiceName}}/Makefile",
		Content: `SERVICE_BIN ?= ../bin/{{.ServiceName}}

.PHONY: help build run test lint verify

help: ## 查看 {{.ServiceName}} 可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "{{.ServiceName}} 可用命令：\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## 构建 {{.ServiceName}} 二进制。
	@mkdir -p $(dir $(SERVICE_BIN))
	go build -o $(SERVICE_BIN) ./cmd

run: ## 运行 {{.ServiceName}}。
	go run ./cmd serve

test: ## 运行 {{.ServiceName}} 测试。
	go test ./...

lint: ## 运行 {{.ServiceName}} lint。
	golangci-lint run ./...

verify: lint test ## 运行 {{.ServiceName}} 验证。
`,
	},
	// 13. README.md
	{
		Path: "{{.ServiceName}}/README.md",
		Content: `# {{.TitleName}}

## 简介

` + "`" + `{{.ServiceName}}` + "`" + ` 是 AegisCore 体系下的业务微服务，与 ` + "`" + `user-service` + "`" + ` 平级组织，依赖 ` + "`" + `platform/` + "`" + ` 共享契约与底层原语。

## 目录结构

` + "```" + `text
{{.ServiceName}}/
├── cmd/               # 命令行入口与 serve 指令
├── internal/
│   ├── bootstrap/     # Fx 依赖注入总装与 HTTP 生命周期
│   ├── config/        # 服务私有配置加载与脱敏
│   ├── providers/     # 基础设施组件与传输层 Provider
│   └── router/        # 路由分组与健康检查探针
├── Makefile           # 服务级构建与测试目标
├── go.mod             # 独立 Go 模块定义
└── README.md          # 模块使用与架构说明
` + "```" + `

## 常用命令

` + "```" + `bash
# 模块级命令
make -C {{.ServiceName}} build
make -C {{.ServiceName}} run
make -C {{.ServiceName}} test
make -C {{.ServiceName}} lint

# 根目录代理执行
make {{.ServiceName}}-build
make {{.ServiceName}}-run
make {{.ServiceName}}-test
` + "```" + `

## 配置与运行

默认监听端口为 ` + "`" + `:{{.Port}}` + "`" + `，健康探针如下：
- ` + "`" + `GET /livez` + "`" + `：存活探针
- ` + "`" + `GET /readyz` + "`" + `：就绪探针
- ` + "`" + `GET /startupz` + "`" + `：启动探针
- ` + "`" + `GET /api/v1/ping` + "`" + `：服务连通性测试
`,
	},
	// 14. Nacos local-host 配置
	{
		Path: "deployments/nacos/local-host/{{.ServiceName}}.yaml",
		Content: `app:
  name: aegiscore-{{.ServiceName}}

server:
  http:
    enabled: true
    host: 0.0.0.0
    port: {{.Port}}
    read_timeout: 30s
    write_timeout: 60s
    idle_timeout: 120s
    shutdown_timeout: 25s

http:
  request_body_max_bytes: 65536
`,
	},
	// 15. Nacos local-docker 配置
	{
		Path: "deployments/nacos/local-docker/{{.ServiceName}}.yaml",
		Content: `app:
  name: aegiscore-{{.ServiceName}}

server:
  http:
    enabled: true
    host: 0.0.0.0
    port: {{.Port}}
    read_timeout: 30s
    write_timeout: 60s
    idle_timeout: 120s
    shutdown_timeout: 25s

http:
  request_body_max_bytes: 65536
`,
	},
	// 16. Dockerfile
	{
		Path: "deployments/docker/{{.ServiceName}}.Dockerfile",
		Content: `# syntax=docker/dockerfile:1.25
FROM golang:1.27.4-alpine3.24@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf AS builder
WORKDIR /src

COPY go.work go.work.sum ./
COPY platform/go.mod platform/go.sum ./platform/
COPY {{.ServiceName}}/go.mod {{.ServiceName}}/go.sum* ./{{.ServiceName}}/

RUN --mount=type=cache,target=/go/pkg/mod \
  go list -m -mod=readonly all >/dev/null || true

COPY platform ./platform
COPY {{.ServiceName}} ./{{.ServiceName}}

WORKDIR /src/{{.ServiceName}}
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w" \
    -o /out/{{.ServiceName}} ./cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35
WORKDIR /app

COPY --from=builder --chmod=0755 /out/{{.ServiceName}} /app/{{.ServiceName}}/bin/{{.ServiceName}}

USER 65532:65532
ENTRYPOINT ["/app/{{.ServiceName}}/bin/{{.ServiceName}}"]
CMD ["serve"]
`,
	},
}
