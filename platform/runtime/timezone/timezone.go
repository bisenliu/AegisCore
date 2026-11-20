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

// init 设置整个系统使用的统一业务时区（例如默认使用 Asia/Shanghai）；如果时区名称不正确则阻止系统启动。
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
	// 同步更新系统全局的本地时区变量（time.Local），确保整个程序在格式化时间、解析时间字符串时统一使用该时区。
	time.Local = location
	s.initialized = true
	return nil
}
