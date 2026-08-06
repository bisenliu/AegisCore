package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/localcache"
)

func TestLocalcacheCollectorExportsStats(t *testing.T) {
	source := staticLocalcacheStatsSource{
		name: "auth_token_version",
		stats: localcache.Stats{
			Hit:               10,
			Miss:              3,
			LoadSuccess:       2,
			LoadError:         1,
			CapacityEvictions: 7,
			Capacity:          1000,
		},
	}
	collector, err := NewLocalcacheCollector(LocalcacheCollectorOptions{Source: source})
	require.NoError(t, err, "NewLocalcacheCollector")
	registry := prometheus.NewRegistry()
	require.NoError(t, registry.Register(collector), "Register")

	families := gatherRegistryFamilies(t, registry)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheRequestsMetricName), map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultHit}, 10)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheRequestsMetricName), map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultMiss}, 3)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheLoadsMetricName), map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultSuccess}, 2)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheLoadsMetricName), map[string]string{LabelCache: "auth_token_version", LabelResult: localcacheResultError}, 1)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheCapacityEvictionsMetricName), map[string]string{LabelCache: "auth_token_version"}, 7)
	assertMetricWithLabelsValue(t, familyFrom(t, families, localcacheCapacityMetricName), map[string]string{LabelCache: "auth_token_version"}, 1000)
	require.Nil(t, findMetricFamily(families, "aegiscore_localcache_singleflight_total"))
	require.Nil(t, findMetricFamily(families, "aegiscore_localcache_writes_total"))
	require.Nil(t, findMetricFamily(families, "aegiscore_localcache_evictions_total"))
}

func findMetricFamily(families []*io_prometheus_client.MetricFamily, name string) *io_prometheus_client.MetricFamily {
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	return nil
}

func TestLocalcacheCollectorAllowsMultipleCaches(t *testing.T) {
	registry := prometheus.NewRegistry()
	for _, source := range []staticLocalcacheStatsSource{
		{name: "session_tokens", stats: localcache.Stats{Hit: 1, Capacity: 100}},
		{name: "catalog_entries", stats: localcache.Stats{Miss: 2, Capacity: 200}},
	} {
		collector, err := NewLocalcacheCollector(LocalcacheCollectorOptions{Source: source})
		require.NoErrorf(t, err, "NewLocalcacheCollector(%s)", source.name)
		require.NoErrorf(t, registry.Register(collector), "Register(%s)", source.name)
	}

	family := familyFrom(t, gatherRegistryFamilies(t, registry), localcacheRequestsMetricName)
	assertMetricWithLabelsValue(t, family, map[string]string{LabelCache: "session_tokens", LabelResult: localcacheResultHit}, 1)
	assertMetricWithLabelsValue(t, family, map[string]string{LabelCache: "catalog_entries", LabelResult: localcacheResultMiss}, 2)
}

type staticLocalcacheStatsSource struct {
	name  string
	stats localcache.Stats
}

func (s staticLocalcacheStatsSource) Name() string {
	return s.name
}

func (s staticLocalcacheStatsSource) Stats() localcache.Stats {
	return s.stats
}

func gatherRegistryFamilies(t *testing.T, gatherer prometheus.Gatherer) []*io_prometheus_client.MetricFamily {
	t.Helper()
	families, err := gatherer.Gather()
	require.NoError(t, err, "Gather")
	return families
}
