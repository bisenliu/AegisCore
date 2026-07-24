package timezone

import (
	"fmt"
	"sync"
	"time"
)

var state timezoneState

type timezoneState struct {
	mu          sync.Mutex
	initialized bool
}

// Init 根据已校验的配置时区初始化进程时区。
func Init(timezone string) error {
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
	// 更新 time.Local，确保标准库格式化和 location 查询语义一致。
	time.Local = location
	s.initialized = true
	return nil
}
