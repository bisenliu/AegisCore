package main

import (
	"testing"
	"time"

	"github.com/aegiscore/common/runtime/config"
)

func TestFxAppLifecycleTimeouts(t *testing.T) {
	if fxAppStartTimeout != 15*time.Second {
		t.Fatalf("fxAppStartTimeout = %s, want 15s", fxAppStartTimeout)
	}

	cfg, err := config.Load("../configs/config.yaml")
	if err != nil {
		t.Fatalf("Load default config: %v", err)
	}
	if fxAppStopTimeout < cfg.HTTP.ShutdownTimeout {
		t.Fatalf("fxAppStopTimeout = %s, want at least configured http.shutdown_timeout %s", fxAppStopTimeout, cfg.HTTP.ShutdownTimeout)
	}
}
