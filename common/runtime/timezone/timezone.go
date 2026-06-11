package timezone

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aegiscore/common/runtime/config"
)

// DefaultTimezone 是配置缺省 system.timezone 时使用的进程时区兜底值。
const DefaultTimezone = "Asia/Shanghai"

var state timezoneState

type timezoneState struct {
	mu          sync.Mutex
	initialized bool
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
