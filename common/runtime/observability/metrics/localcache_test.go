package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"

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

	families := gatherRegistryFamilies(t, registry)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheRequestsMetricName), map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultHit}, 10)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheRequestsMetricName), map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultMiss}, 3)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheLoadsMetricName), map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultSuccess}, 1)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheLoadsMetricName), map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultError}, 1)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheSingleflightMetricName), map[string]string{LabelCache: "auth_token_version", LabelEvent: localcacheEventShared}, 4)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheSingleflightMetricName), map[string]string{LabelCache: "auth_token_version", LabelEvent: localcacheEventDoubleCheck}, 1)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheWritesMetricName), map[string]string{LabelCache: "auth_token_version", LabelEvent: localcacheEventSetDropped}, 5)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheWritesMetricName), map[string]string{LabelCache: "auth_token_version", LabelEvent: localcacheEventRejected}, 6)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheEvictionsMetricName), map[string]string{LabelCache: "auth_token_version"}, 7)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheCapacityMetricName), map[string]string{LabelCache: "auth_token_version"}, 1000)
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

	family := familyFrom(t, gatherRegistryFamilies(t, registry), localcacheRequestsMetricName)
	assertMetricWithLabelsValue(t, family, map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultHit}, 1)
	assertMetricWithLabelsValue(t, family, map[string]string{LabelCache: "rbac_user_roles", LabelResult: localcacheResultMiss}, 2)
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

func gatherRegistryFamilies(t *testing.T, gatherer prometheus.Gatherer) []*io_prometheus_client.MetricFamily {
	t.Helper()
	families, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	return families
}
