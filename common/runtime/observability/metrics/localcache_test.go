package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/aegiscore/common/runtime/localcache"
)

func TestLocalcacheCollectorExportsStats(t *testing.T) {
	source := fakeLocalcacheStatsSource{
		name: "auth_token_version",
		stats: localcache.Stats{
			Hit:            10,
			Miss:           3,
			Load:           2,
			LoadError:      1,
			Shared:         4,
			DoubleCheckHit: 1,
			SetDropped:     5,
			Rejected:       6,
			Evicted:        7,
			Capacity:       1000,
		},
	}
	collector, err := NewLocalcacheCollector(LocalcacheCollectorOptions{Source: source})
	if err != nil {
		t.Fatalf("NewLocalcacheCollector: %v", err)
	}
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("Register: %v", err)
	}

	body := gatherRegistryText(t, registry)
	for _, want := range []string{
		`aegiscore_localcache_requests_total{cache="auth_token_version",result="hit"} 10`,
		`aegiscore_localcache_requests_total{cache="auth_token_version",result="miss"} 3`,
		`aegiscore_localcache_loads_total{cache="auth_token_version",result="success"} 1`,
		`aegiscore_localcache_loads_total{cache="auth_token_version",result="error"} 1`,
		`aegiscore_localcache_singleflight_total{cache="auth_token_version",event="shared"} 4`,
		`aegiscore_localcache_singleflight_total{cache="auth_token_version",event="double_check_hit"} 1`,
		`aegiscore_localcache_writes_total{cache="auth_token_version",event="set_dropped"} 5`,
		`aegiscore_localcache_writes_total{cache="auth_token_version",event="rejected"} 6`,
		`aegiscore_localcache_evictions_total{cache="auth_token_version"} 7`,
		`aegiscore_localcache_capacity{cache="auth_token_version"} 1000`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
}

func TestLocalcacheCollectorAllowsMultipleCaches(t *testing.T) {
	registry := prometheus.NewRegistry()
	for _, source := range []fakeLocalcacheStatsSource{
		{name: "auth_token_version", stats: localcache.Stats{Hit: 1, Capacity: 100}},
		{name: "rbac_user_roles", stats: localcache.Stats{Miss: 2, Capacity: 200}},
	} {
		collector, err := NewLocalcacheCollector(LocalcacheCollectorOptions{Source: source})
		if err != nil {
			t.Fatalf("NewLocalcacheCollector(%s): %v", source.name, err)
		}
		if err := registry.Register(collector); err != nil {
			t.Fatalf("Register(%s): %v", source.name, err)
		}
	}

	body := gatherRegistryText(t, registry)
	for _, want := range []string{
		`aegiscore_localcache_requests_total{cache="auth_token_version",result="hit"} 1`,
		`aegiscore_localcache_requests_total{cache="rbac_user_roles",result="miss"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
}

type fakeLocalcacheStatsSource struct {
	name  string
	stats localcache.Stats
}

func (s fakeLocalcacheStatsSource) Name() string {
	return s.name
}

func (s fakeLocalcacheStatsSource) Stats() localcache.Stats {
	return s.stats
}

func gatherRegistryText(t *testing.T, gatherer prometheus.Gatherer) string {
	t.Helper()
	families, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	var builder strings.Builder
	for _, family := range families {
		if _, err := expfmt.MetricFamilyToText(&builder, family); err != nil {
			t.Fatalf("MetricFamilyToText: %v", err)
		}
	}
	return builder.String()
}
