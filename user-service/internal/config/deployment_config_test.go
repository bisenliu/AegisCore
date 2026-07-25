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
		Healthcheck struct {
			Test []string `yaml:"test"`
		} `yaml:"healthcheck"`
	} `yaml:"services"`
}

type deploymentResourcesDocument struct {
	Resources struct {
		Postgres map[string]struct {
			Username string `yaml:"username"`
			DBName   string `yaml:"db_name"`
		} `yaml:"postgres"`
	} `yaml:"resources"`
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
	resourcesPath := filepath.Join(repoRoot, "deployments", "compose", "nacos", "init", "resources.yaml")
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

	resources := readDeploymentYAML[deploymentResourcesDocument](t, resourcesPath)
	primaryDB, ok := resources.Resources.Postgres["primary_db"]
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
