package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go.uber.org/fx"
	"go.uber.org/zap"

	commonpprof "github.com/aegiscore/common/http/pprof"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
)

const (
	pprofEnabledEnv  = "PPROF_ENABLED"
	pprofAddrEnv     = "PPROF_ADDR"
	defaultPprofAddr = "127.0.0.1:6060"
)

type pprofSettings struct {
	enabled bool
	addr    string
}

// PprofServer 持有独立于业务 Gin router 的诊断 HTTP server。
type PprofServer struct {
	Server  *http.Server
	Enabled bool
}

// PprofServerParams 包含诊断 server 的进程环境、安全校验和生命周期依赖。
type PprofServerParams struct {
	fx.In

	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Config     *config.Config
	Log        *zap.Logger
}

// NewPprofServer 从进程环境读取诊断监听设置，并仅在启用时注册独立生命周期。
func NewPprofServer(params PprofServerParams) (*PprofServer, error) {
	settings, err := loadPprofSettings(os.LookupEnv)
	if err != nil {
		return nil, err
	}
	environment := ""
	if params.Config != nil {
		environment = params.Config.App.Environment
	}
	if settings.enabled && isProductionLike(environment) && !isLoopbackPprofAddr(settings.addr) {
		return nil, fmt.Errorf("%s must use a loopback address in production-like environments", pprofAddrEnv)
	}

	server := &http.Server{
		Addr:    settings.addr,
		Handler: commonpprof.Handler(commonpprof.Options{}),
	}
	result := &PprofServer{Server: server, Enabled: settings.enabled}
	if !settings.enabled {
		return result, nil
	}

	pprofLog := logger.NamedComponent(params.Log, "pprof", "diagnostics")
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			listener, err := net.Listen("tcp", settings.addr)
			if err != nil {
				return fmt.Errorf("listen pprof server on %s: %w", settings.addr, err)
			}
			logger.WithContext(ctx, pprofLog).Info("starting pprof server", zap.String("addr", settings.addr))
			go servePprofServer(pprofLog, params.Shutdowner, server, listener)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.WithContext(ctx, pprofLog).Info("stopping pprof server")
			if err := server.Shutdown(ctx); err != nil {
				return fmt.Errorf("shutdown pprof server: %w", err)
			}
			return nil
		},
	})
	return result, nil
}

func loadPprofSettings(lookup func(string) (string, bool)) (pprofSettings, error) {
	settings := pprofSettings{addr: defaultPprofAddr}
	if value, ok := lookup(pprofEnabledEnv); ok {
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return pprofSettings{}, fmt.Errorf("parse %s: %w", pprofEnabledEnv, err)
		}
		settings.enabled = enabled
	}
	if value, ok := lookup(pprofAddrEnv); ok {
		settings.addr = strings.TrimSpace(value)
	}
	if err := validatePprofAddr(settings.addr); err != nil {
		return pprofSettings{}, fmt.Errorf("validate %s: %w", pprofAddrEnv, err)
	}
	return settings, nil
}

func validatePprofAddr(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("must be a host:port address")
	}
	if strings.TrimSpace(host) == "" {
		return errors.New("host is required")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func isLoopbackPprofAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}

func isProductionLike(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "prod", "production", "staging":
		return true
	default:
		return false
	}
}

func servePprofServer(log *zap.Logger, shutdowner fx.Shutdowner, server *http.Server, listener net.Listener) {
	err := server.Serve(listener)
	if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return
	}
	log.Error("pprof server failed", logger.StackTrace(zap.Error(err))...)
	if shutdowner != nil {
		if shutdownErr := shutdowner.Shutdown(); shutdownErr != nil {
			log.Error("shutdown after pprof server failure failed", logger.StackTrace(zap.Error(shutdownErr))...)
		}
	}
}
