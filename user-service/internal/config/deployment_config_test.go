package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type deploymentDatabaseTarget struct {
	username string
	database string
}

type deploymentNacosEndpoint struct {
	serviceName   string
	namespace     string
	clusterDomain string
	port          int
}

type deploymentComposeDocument struct {
	Services map[string]struct {
		Environment map[string]string `yaml:"environment"`
		Ports       []string          `yaml:"ports"`
		Volumes     []string          `yaml:"volumes"`
		Restart     string            `yaml:"restart"`
		DependsOn   map[string]struct {
			Condition string `yaml:"condition"`
		} `yaml:"depends_on"`
		Healthcheck struct {
			Test []string `yaml:"test"`
		} `yaml:"healthcheck"`
	} `yaml:"services"`
}

type deploymentResourcesDocument struct {
	Resources struct {
		Redis map[string]struct {
			Mode  string   `yaml:"mode"`
			Addr  string   `yaml:"addr"`
			Addrs []string `yaml:"addrs"`
		} `yaml:"redis"`
		Postgres map[string]struct {
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			Username string `yaml:"username"`
			DBName   string `yaml:"db_name"`
		} `yaml:"postgres"`
	} `yaml:"resources"`
}

type deploymentBaseDocument struct {
	Observability struct {
		Tracing struct {
			OTLPEndpoint string `yaml:"otlp_endpoint"`
		} `yaml:"tracing"`
	} `yaml:"observability"`
}

type deploymentWorkloadDocument struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Env []struct {
						Name  string `yaml:"name"`
						Value string `yaml:"value"`
					} `yaml:"env"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type deploymentNetworkPolicyDocument struct {
	Spec struct {
		Egress []struct {
			To []struct {
				NamespaceSelector struct {
					MatchLabels map[string]string `yaml:"matchLabels"`
				} `yaml:"namespaceSelector"`
				PodSelector struct {
					MatchLabels map[string]string `yaml:"matchLabels"`
				} `yaml:"podSelector"`
			} `yaml:"to"`
			Ports []struct {
				Port int `yaml:"port"`
			} `yaml:"ports"`
		} `yaml:"egress"`
	} `yaml:"spec"`
}

type deploymentHelmValuesDocument struct {
	Nacos struct {
		Service string `yaml:"service"`
		Server  struct {
			ServiceName   string            `yaml:"serviceName"`
			Namespace     string            `yaml:"namespace"`
			ClusterDomain string            `yaml:"clusterDomain"`
			Port          int               `yaml:"port"`
			PodSelector   map[string]string `yaml:"podSelector"`
		} `yaml:"server"`
		ConfigNamespace string `yaml:"configNamespace"`
		Group           string `yaml:"group"`
	} `yaml:"nacos"`
}

func TestRepositoryDeploymentConfigConsistency(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")

	composePath := filepath.Join(repoRoot, "deployments", "compose", "docker-compose.yml")
	resourcesPath := filepath.Join(repoRoot, "deployments", "nacos", "local-docker", "resources.yaml")
	nacosDir := filepath.Join(repoRoot, "deployments", "nacos")
	metricsPath := filepath.Join(repoRoot, "deployments", "compose", "scripts", "generate-real-metrics-load.sh")
	kubernetesDir := filepath.Join(repoRoot, "deployments", "k8s", "user-service")
	chartDir := filepath.Join(repoRoot, "deployments", "helm", "aegiscore-user-service")

	compose := readDeploymentYAML[deploymentComposeDocument](t, composePath)
	postgres, ok := compose.Services["postgres"]
	require.True(t, ok, "compose postgres service is required")
	composeTarget := deploymentDatabaseTarget{
		username: strings.TrimSpace(postgres.Environment["POSTGRES_USER"]),
		database: strings.TrimSpace(postgres.Environment["POSTGRES_DB"]),
	}
	require.NotEmpty(t, composeTarget.username)
	require.NotEmpty(t, composeTarget.database)
	require.Equal(t, composeTarget.username, deploymentFlagValue(t, postgres.Healthcheck.Test, "-U"))
	require.Equal(t, composeTarget.database, deploymentFlagValue(t, postgres.Healthcheck.Test, "-d"))

	redis, ok := compose.Services["redis"]
	require.True(t, ok, "compose redis service is required")
	hostInit, ok := compose.Services["nacos-init-host"]
	require.True(t, ok, "compose nacos-init-host service is required")
	require.Contains(t, hostInit.Volumes, "../nacos/local-host:/nacos/config:ro")
	require.Equal(t, "loca-host", hostInit.Environment["AEGISCORE_NACOS_NAMESPACE"])
	require.Equal(t, "AEGISCORE", hostInit.Environment["AEGISCORE_NACOS_GROUP"])
	require.Empty(t, hostInit.Environment["AEGISCORE_NACOS_DATA_IDS"], "Compose 应使用运行时默认三文档顺序")
	require.Equal(t, "no", hostInit.Restart)

	dockerInit, ok := compose.Services["nacos-init-docker"]
	require.True(t, ok, "compose nacos-init-docker service is required")
	require.Contains(t, dockerInit.Volumes, "../nacos/local-docker:/nacos/config:ro")
	require.Equal(t, "loca-docker", dockerInit.Environment["AEGISCORE_NACOS_NAMESPACE"])
	require.Equal(t, "AEGISCORE", dockerInit.Environment["AEGISCORE_NACOS_GROUP"])
	require.Empty(t, dockerInit.Environment["AEGISCORE_NACOS_DATA_IDS"], "Compose 应使用运行时默认三文档顺序")
	require.Equal(t, "no", dockerInit.Restart)
	requireDeploymentConfigDocuments(t, filepath.Join(nacosDir, "local-docker"))
	requireDeploymentConfigDocuments(t, filepath.Join(nacosDir, "local-host"))

	for _, serviceName := range []string{"user-service", "rbac-seed"} {
		service, exists := compose.Services[serviceName]
		require.True(t, exists, "compose %s service is required", serviceName)
		require.Equal(t, "loca-docker", service.Environment["AEGISCORE_NACOS_NAMESPACE"])
		require.Empty(t, service.Environment["AEGISCORE_NACOS_DATA_IDS"], "Compose 应使用运行时默认三文档顺序")
		require.Equal(t, "service_completed_successfully", service.DependsOn["nacos-init-docker"].Condition)
	}

	hostResources := readDeploymentYAML[deploymentResourcesDocument](t, filepath.Join(nacosDir, "local-host", "resources.yaml"))
	dockerResources := readDeploymentYAML[deploymentResourcesDocument](t, resourcesPath)
	hostBase := readDeploymentYAML[deploymentBaseDocument](t, filepath.Join(nacosDir, "local-host", "base.yaml"))
	dockerBase := readDeploymentYAML[deploymentBaseDocument](t, filepath.Join(nacosDir, "local-docker", "base.yaml"))
	require.Equal(t, "standalone", hostResources.Resources.Redis["cache_redis"].Mode)
	require.Equal(t, "127.0.0.1:"+deploymentPublishedPort(t, redis.Ports, "6379"), hostResources.Resources.Redis["cache_redis"].Addr)
	require.Empty(t, hostResources.Resources.Redis["cache_redis"].Addrs)
	require.Equal(t, "127.0.0.1", hostResources.Resources.Postgres["primary_db"].Host)
	require.Equal(t, deploymentPublishedPortInt(t, postgres.Ports, "5432"), hostResources.Resources.Postgres["primary_db"].Port)
	require.Equal(t, "127.0.0.1:4317", hostBase.Observability.Tracing.OTLPEndpoint)
	require.Equal(t, "standalone", dockerResources.Resources.Redis["cache_redis"].Mode)
	require.Equal(t, "redis:6379", dockerResources.Resources.Redis["cache_redis"].Addr)
	require.Empty(t, dockerResources.Resources.Redis["cache_redis"].Addrs)
	require.Equal(t, "postgres", dockerResources.Resources.Postgres["primary_db"].Host)
	require.Equal(t, 5432, dockerResources.Resources.Postgres["primary_db"].Port)
	require.Equal(t, "jaeger:4317", dockerBase.Observability.Tracing.OTLPEndpoint)

	primaryDB, ok := dockerResources.Resources.Postgres["primary_db"]
	require.True(t, ok, "resources.postgres.primary_db is required")
	require.Equal(t, composeTarget, deploymentDatabaseTarget{
		username: strings.TrimSpace(primaryDB.Username),
		database: strings.TrimSpace(primaryDB.DBName),
	})

	metrics := readDeploymentFile(t, metricsPath)
	require.Equal(t, composeTarget.username, deploymentShellVariable(t, metrics, "POSTGRES_USER"))
	require.Equal(t, composeTarget.database, deploymentShellVariable(t, metrics, "POSTGRES_DB"))

	deploymentEndpoint := deploymentWorkloadNacosEndpoint(t, filepath.Join(kubernetesDir, "deployment.yaml"))
	seedEndpoint := deploymentWorkloadNacosEndpoint(t, filepath.Join(kubernetesDir, "rbac-seed-job.yaml"))
	require.Equal(t, deploymentEndpoint, seedEndpoint)
	deploymentRequireNetworkPolicyAllowsNacos(t, filepath.Join(kubernetesDir, "networkpolicy.yaml"), deploymentEndpoint)

	helmValues := readDeploymentYAML[deploymentHelmValuesDocument](t, filepath.Join(chartDir, "values.yaml"))
	helmEndpoint := deploymentNacosEndpoint{
		serviceName:   strings.TrimSpace(helmValues.Nacos.Server.ServiceName),
		namespace:     strings.TrimSpace(helmValues.Nacos.Server.Namespace),
		clusterDomain: strings.TrimSpace(helmValues.Nacos.Server.ClusterDomain),
		port:          helmValues.Nacos.Server.Port,
	}
	require.Equal(t, deploymentEndpoint, helmEndpoint)
	require.Equal(t, deploymentEndpoint.serviceName, helmValues.Nacos.Server.PodSelector["app.kubernetes.io/name"])
	require.NotEmpty(t, strings.TrimSpace(helmValues.Nacos.Service))
	require.NotEmpty(t, strings.TrimSpace(helmValues.Nacos.ConfigNamespace))
	require.NotEmpty(t, strings.TrimSpace(helmValues.Nacos.Group))

	helpers := readDeploymentFile(t, filepath.Join(chartDir, "templates", "_helpers.tpl"))
	require.Contains(t, helpers, `.Values.nacos.server.serviceName`)
	require.Contains(t, helpers, `.Values.nacos.server.namespace`)
	require.Contains(t, helpers, `.Values.nacos.server.clusterDomain`)
	require.Contains(t, helpers, `.Values.nacos.server.port`)

	networkPolicy := readDeploymentFile(t, filepath.Join(chartDir, "templates", "networkpolicy.yaml"))
	require.Contains(t, networkPolicy, `.Values.nacos.server.namespace`)
	require.Contains(t, networkPolicy, `.Values.nacos.server.port`)
	require.Contains(t, networkPolicy, `.Values.nacos.server.podSelector`)

	for _, templateName := range []string{"deployment.yaml", "rbac-seed-job.yaml"} {
		template := readDeploymentFile(t, filepath.Join(chartDir, "templates", templateName))
		require.Contains(t, template, `include "aegiscore-user-service.nacosEnv" .`)
	}
}

func requireDeploymentConfigDocuments(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	require.ElementsMatch(t, []string{"base.yaml", "resources.yaml", "user-service.yaml"}, names)
}

func deploymentPublishedPort(t *testing.T, ports []string, containerPort string) string {
	t.Helper()
	for _, mapping := range ports {
		hostPort, targetPort, ok := strings.Cut(mapping, ":")
		if ok && targetPort == containerPort {
			return hostPort
		}
	}
	require.FailNow(t, "compose published port is missing", "container_port=%s", containerPort)
	return ""
}

func deploymentPublishedPortInt(t *testing.T, ports []string, containerPort string) int {
	t.Helper()
	value, err := strconv.Atoi(deploymentPublishedPort(t, ports, containerPort))
	require.NoError(t, err)
	return value
}

func readDeploymentYAML[T any](t *testing.T, path string) T {
	t.Helper()
	var document T
	require.NoError(t, yaml.Unmarshal([]byte(readDeploymentFile(t, path)), &document), "parse YAML %s", path)
	return document
}

func readDeploymentFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)
	return string(content)
}

func deploymentFlagValue(t *testing.T, arguments []string, name string) string {
	t.Helper()
	for index, argument := range arguments {
		if argument == name {
			require.Less(t, index+1, len(arguments), "%s requires a value", name)
			return strings.TrimSpace(arguments[index+1])
		}
	}
	require.FailNow(t, "required healthcheck flag is missing", "flag=%s", name)
	return ""
}

func deploymentShellVariable(t *testing.T, script string, name string) string {
	t.Helper()
	prefix := name + "="
	for _, line := range strings.Split(script, "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), prefix)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			return strings.TrimSpace(unquoted)
		}
		return strings.Trim(strings.TrimSpace(value), "'")
	}
	require.FailNow(t, "required shell variable is missing", "variable=%s", name)
	return ""
}

func deploymentWorkloadNacosEndpoint(t *testing.T, path string) deploymentNacosEndpoint {
	t.Helper()
	document := readDeploymentYAML[deploymentWorkloadDocument](t, path)
	require.NotEmpty(t, document.Spec.Template.Spec.Containers, "%s must contain a container", path)
	for _, variable := range document.Spec.Template.Spec.Containers[0].Env {
		if variable.Name == "AEGISCORE_NACOS_ADDR" {
			endpoint, err := parseDeploymentNacosEndpoint(variable.Value)
			require.NoError(t, err, "parse AEGISCORE_NACOS_ADDR in %s", path)
			return endpoint
		}
	}
	require.FailNow(t, "AEGISCORE_NACOS_ADDR is required", "path=%s", path)
	return deploymentNacosEndpoint{}
}

func parseDeploymentNacosEndpoint(address string) (deploymentNacosEndpoint, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return deploymentNacosEndpoint{}, fmt.Errorf("split Nacos address %q: %w", address, err)
	}
	labels := strings.Split(host, ".")
	if len(labels) < 5 || labels[2] != "svc" {
		return deploymentNacosEndpoint{}, fmt.Errorf("Nacos address %q must use service.namespace.svc.cluster-domain", address)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 {
		return deploymentNacosEndpoint{}, fmt.Errorf("Nacos address %q has invalid port", address)
	}
	return deploymentNacosEndpoint{
		serviceName:   labels[0],
		namespace:     labels[1],
		clusterDomain: strings.Join(labels[3:], "."),
		port:          port,
	}, nil
}

func deploymentRequireNetworkPolicyAllowsNacos(t *testing.T, path string, endpoint deploymentNacosEndpoint) {
	t.Helper()
	document := readDeploymentYAML[deploymentNetworkPolicyDocument](t, path)
	for _, egress := range document.Spec.Egress {
		for _, destination := range egress.To {
			if destination.PodSelector.MatchLabels["app.kubernetes.io/name"] != endpoint.serviceName {
				continue
			}
			require.Equal(t, endpoint.namespace, destination.NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"])
			for _, port := range egress.Ports {
				if port.Port == endpoint.port {
					return
				}
			}
			require.FailNow(t, "Nacos NetworkPolicy egress port is missing", "port=%d", endpoint.port)
		}
	}
	require.FailNow(t, "Nacos NetworkPolicy egress destination is missing", "service=%s", endpoint.serviceName)
}
