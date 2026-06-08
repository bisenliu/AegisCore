package timezone

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aegiscore/common/runtime/config"
	"go.uber.org/fx"
)

// DefaultTimezone 是配置缺省 system.timezone 时使用的进程时区兜底值。
const DefaultTimezone = "Asia/Shanghai"

var state timezoneState

type timezoneState struct {
	mu          sync.Mutex
	initialized bool
}

// Params 包含初始化进程时区所需的 Fx 依赖。
type Params struct {
	fx.In

	Config *config.Config
}

// Init 根据 Fx 提供的配置初始化进程时区。
func Init(params Params) error {
	return InitConfig(params.Config)
}

// InitConfig 根据 config 初始化进程时区，并在缺省时回退到 DefaultTimezone。
func InitConfig(cfg *config.Config) error {
	timezone := DefaultTimezone
	if cfg != nil {
		configured := strings.TrimSpace(cfg.System.Timezone)
		if configured != "" {
			timezone = configured
		}
	}

	return state.init(timezone)
}

func (s *timezoneState) init(timezone string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		// 成功初始化是进程级状态；失败尝试会保持 initialized 为 false，允许后续重试。
		return nil
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	if err := os.Setenv("TZ", timezone); err != nil {
		return fmt.Errorf("set TZ %q: %w", timezone, err)
	}
	// 同时更新 TZ 和 time.Local，确保标准库格式化和 location 查询语义一致。
	time.Local = location
	s.initialized = true
	return nil
}

// Module 在 Fx 启动阶段初始化进程时区。
var Module = fx.Module("aegiscore-common-timezone",
	fx.Invoke(Init),
)
