package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
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
			require.Equalf(t, tt.want, StatusClass(tt.status), "StatusClass(%d)", tt.status)
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
		require.Equalf(t, wants[name], got, "%s", name)
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
		require.NoErrorf(t, ValidateLowCardinalityLabelKey(key), "ValidateLowCardinalityLabelKey(%q)", key)
	}

	require.ErrorIs(t, ValidateLowCardinalityLabelKey("user_id"), ErrUnsupportedLabelKey)
}
