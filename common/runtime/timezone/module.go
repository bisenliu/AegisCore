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

const DefaultTimezone = "Asia/Shanghai"

var state timezoneState

type timezoneState struct {
	mu          sync.Mutex
	initialized bool
}

type Params struct {
	fx.In

	Config *config.Config
}

func Init(params Params) error {
	return InitConfig(params.Config)
}

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
		return nil
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	if err := os.Setenv("TZ", timezone); err != nil {
		return fmt.Errorf("set TZ %q: %w", timezone, err)
	}
	time.Local = location
	s.initialized = true
	return nil
}

var Module = fx.Module("aegiscore-common-timezone",
	fx.Invoke(Init),
)
