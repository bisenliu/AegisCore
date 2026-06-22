package metrics

import (
	"errors"
	"testing"
)

func TestStatusClass(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: 99, want: "unknown"},
		{status: 100, want: "1xx"},
		{status: 200, want: "2xx"},
		{status: 204, want: "2xx"},
		{status: 302, want: "3xx"},
		{status: 404, want: "4xx"},
		{status: 500, want: "5xx"},
		{status: 599, want: "5xx"},
		{status: 600, want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := StatusClass(tt.status); got != tt.want {
				t.Fatalf("StatusClass(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestLabelKeyConstants(t *testing.T) {
	tests := map[string]string{
		"LabelService":      LabelService,
		"LabelEnvironment":  LabelEnvironment,
		"LabelMethod":       LabelMethod,
		"LabelRoute":        LabelRoute,
		"LabelStatusClass":  LabelStatusClass,
		"LabelCode":         LabelCode,
		"LabelResult":       LabelResult,
		"LabelResource":     LabelResource,
		"LabelCache":        LabelCache,
		"LabelPool":         LabelPool,
		"LabelSchedulerJob": LabelSchedulerJob,
		"LabelEvent":        LabelEvent,
		"LabelStatus":       LabelStatus,
		"LabelReason":       LabelReason,
	}
	wants := map[string]string{
		"LabelService":      "service",
		"LabelEnvironment":  "environment",
		"LabelMethod":       "method",
		"LabelRoute":        "route",
		"LabelStatusClass":  "status_class",
		"LabelCode":         "code",
		"LabelResult":       "result",
		"LabelResource":     "resource",
		"LabelCache":        "cache",
		"LabelPool":         "pool",
		"LabelSchedulerJob": "scheduler_job",
		"LabelEvent":        "event",
		"LabelStatus":       "status",
		"LabelReason":       "reason",
	}
	for name, got := range tests {
		if got != wants[name] {
			t.Fatalf("%s = %q, want %q", name, got, wants[name])
		}
	}
}

func TestValidateLowCardinalityLabelKey(t *testing.T) {
	for _, key := range []string{
		LabelService,
		LabelEnvironment,
		LabelMethod,
		LabelRoute,
		LabelStatusClass,
		LabelCode,
		LabelResult,
		LabelResource,
		LabelCache,
		LabelPool,
		LabelSchedulerJob,
		LabelEvent,
		LabelStatus,
		LabelReason,
	} {
		if err := ValidateLowCardinalityLabelKey(key); err != nil {
			t.Fatalf("ValidateLowCardinalityLabelKey(%q): %v", key, err)
		}
	}

	if err := ValidateLowCardinalityLabelKey("user_id"); !errors.Is(err, ErrUnsupportedLabelKey) {
		t.Fatalf("ValidateLowCardinalityLabelKey(user_id) = %v, want ErrUnsupportedLabelKey", err)
	}
}
