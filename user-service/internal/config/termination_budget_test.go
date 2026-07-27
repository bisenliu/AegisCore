package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const minimumTerminationPlatformMargin = 30 * time.Second

type terminationBudgets struct {
	stopTimeout     time.Duration
	kubernetesGrace time.Duration
	helmGrace       time.Duration
}

func TestRepositoryTerminationBudgets(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	budgets, err := loadTerminationBudgets(
		filepath.Join(repoRoot, "deployments", "nacos", "local-docker", "base.yaml"),
		filepath.Join(repoRoot, "deployments", "k8s", "user-service", "deployment.yaml"),
		filepath.Join(repoRoot, "deployments", "helm", "aegiscore-user-service", "values.yaml"),
	)
	require.NoError(t, err)
	require.Equal(t, 120*time.Second, budgets.stopTimeout)
	require.Equal(t, 150*time.Second, budgets.kubernetesGrace)
	require.Equal(t, 150*time.Second, budgets.helmGrace)
	require.NoError(t, validateTerminationBudgets(budgets, minimumTerminationPlatformMargin))
}

func TestParseTerminationBudgetsRejectsInvalidInput(t *testing.T) {
	validApp := []byte("runtime:\n  lifecycle:\n    stop_timeout: 120s\n")
	validKubernetes := []byte("spec:\n  template:\n    spec:\n      terminationGracePeriodSeconds: 150\n")
	validHelm := []byte("deployment:\n  terminationGracePeriodSeconds: 150\n")

	tests := []struct {
		name           string
		app            []byte
		kubernetes     []byte
		helm           []byte
		errorSubstring string
	}{
		{
			name:           "missing stop timeout",
			app:            []byte("runtime:\n  lifecycle: {}\n"),
			kubernetes:     validKubernetes,
			helm:           validHelm,
			errorSubstring: "user-service runtime.lifecycle.stop_timeout is required",
		},
		{
			name:           "invalid stop timeout",
			app:            []byte("runtime:\n  lifecycle:\n    stop_timeout: invalid\n"),
			kubernetes:     validKubernetes,
			helm:           validHelm,
			errorSubstring: "parse user-service runtime.lifecycle.stop_timeout",
		},
		{
			name:           "missing kubernetes grace",
			app:            validApp,
			kubernetes:     []byte("spec:\n  template:\n    spec: {}\n"),
			helm:           validHelm,
			errorSubstring: "kubernetes spec.template.spec.terminationGracePeriodSeconds is required",
		},
		{
			name:           "non-positive kubernetes grace",
			app:            validApp,
			kubernetes:     []byte("spec:\n  template:\n    spec:\n      terminationGracePeriodSeconds: 0\n"),
			helm:           validHelm,
			errorSubstring: "kubernetes spec.template.spec.terminationGracePeriodSeconds must be > 0",
		},
		{
			name:           "missing helm grace",
			app:            validApp,
			kubernetes:     validKubernetes,
			helm:           []byte("deployment: {}\n"),
			errorSubstring: "helm deployment.terminationGracePeriodSeconds is required",
		},
		{
			name:           "non-positive helm grace",
			app:            validApp,
			kubernetes:     validKubernetes,
			helm:           []byte("deployment:\n  terminationGracePeriodSeconds: -1\n"),
			errorSubstring: "helm deployment.terminationGracePeriodSeconds must be > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTerminationBudgets(tt.app, tt.kubernetes, tt.helm)
			require.ErrorContains(t, err, tt.errorSubstring)
		})
	}
}

func TestValidateTerminationBudgetsRejectsInsufficientGraceAndDrift(t *testing.T) {
	tests := []struct {
		name            string
		budgets         terminationBudgets
		errorSubstrings []string
	}{
		{
			name: "kubernetes grace below minimum",
			budgets: terminationBudgets{
				stopTimeout:     120 * time.Second,
				kubernetesGrace: 149 * time.Second,
				helmGrace:       150 * time.Second,
			},
			errorSubstrings: []string{
				"kubernetes termination grace 2m29s is less than required 2m30s",
				"termination grace drift: kubernetes=2m29s helm=2m30s",
			},
		},
		{
			name: "helm grace below minimum",
			budgets: terminationBudgets{
				stopTimeout:     120 * time.Second,
				kubernetesGrace: 150 * time.Second,
				helmGrace:       35 * time.Second,
			},
			errorSubstrings: []string{
				"helm termination grace 35s is less than required 2m30s",
				"termination grace drift: kubernetes=2m30s helm=35s",
			},
		},
		{
			name: "deployment defaults drift above minimum",
			budgets: terminationBudgets{
				stopTimeout:     120 * time.Second,
				kubernetesGrace: 150 * time.Second,
				helmGrace:       151 * time.Second,
			},
			errorSubstrings: []string{
				"termination grace drift: kubernetes=2m30s helm=2m31s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTerminationBudgets(tt.budgets, minimumTerminationPlatformMargin)
			for _, substring := range tt.errorSubstrings {
				require.ErrorContains(t, err, substring)
			}
		})
	}
}

func loadTerminationBudgets(appPath string, kubernetesPath string, helmPath string) (terminationBudgets, error) {
	app, err := os.ReadFile(appPath)
	if err != nil {
		return terminationBudgets{}, fmt.Errorf("read user-service config %s: %w", appPath, err)
	}
	kubernetes, err := os.ReadFile(kubernetesPath)
	if err != nil {
		return terminationBudgets{}, fmt.Errorf("read kubernetes deployment %s: %w", kubernetesPath, err)
	}
	helm, err := os.ReadFile(helmPath)
	if err != nil {
		return terminationBudgets{}, fmt.Errorf("read helm values %s: %w", helmPath, err)
	}
	return parseTerminationBudgets(app, kubernetes, helm)
}

func parseTerminationBudgets(app []byte, kubernetes []byte, helm []byte) (terminationBudgets, error) {
	stopTimeout, err := parseStopTimeout(app)
	if err != nil {
		return terminationBudgets{}, err
	}
	kubernetesGrace, err := parseKubernetesGrace(kubernetes)
	if err != nil {
		return terminationBudgets{}, err
	}
	helmGrace, err := parseHelmGrace(helm)
	if err != nil {
		return terminationBudgets{}, err
	}
	return terminationBudgets{
		stopTimeout:     stopTimeout,
		kubernetesGrace: kubernetesGrace,
		helmGrace:       helmGrace,
	}, nil
}

func parseStopTimeout(data []byte) (time.Duration, error) {
	var document struct {
		Runtime struct {
			Lifecycle struct {
				StopTimeout string `yaml:"stop_timeout"`
			} `yaml:"lifecycle"`
		} `yaml:"runtime"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return 0, fmt.Errorf("parse user-service config YAML: %w", err)
	}
	value := strings.TrimSpace(document.Runtime.Lifecycle.StopTimeout)
	if value == "" {
		return 0, errors.New("user-service runtime.lifecycle.stop_timeout is required")
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse user-service runtime.lifecycle.stop_timeout %q: %w", value, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("user-service runtime.lifecycle.stop_timeout must be > 0: %s", duration)
	}
	return duration, nil
}

func parseKubernetesGrace(data []byte) (time.Duration, error) {
	var document struct {
		Spec struct {
			Template struct {
				Spec struct {
					TerminationGracePeriodSeconds *int64 `yaml:"terminationGracePeriodSeconds"`
				} `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return 0, fmt.Errorf("parse kubernetes deployment YAML: %w", err)
	}
	return graceDuration("kubernetes spec.template.spec.terminationGracePeriodSeconds", document.Spec.Template.Spec.TerminationGracePeriodSeconds)
}

func parseHelmGrace(data []byte) (time.Duration, error) {
	var document struct {
		Deployment struct {
			TerminationGracePeriodSeconds *int64 `yaml:"terminationGracePeriodSeconds"`
		} `yaml:"deployment"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return 0, fmt.Errorf("parse helm values YAML: %w", err)
	}
	return graceDuration("helm deployment.terminationGracePeriodSeconds", document.Deployment.TerminationGracePeriodSeconds)
}

func graceDuration(source string, seconds *int64) (time.Duration, error) {
	if seconds == nil {
		return 0, fmt.Errorf("%s is required", source)
	}
	if *seconds <= 0 {
		return 0, fmt.Errorf("%s must be > 0: %d", source, *seconds)
	}
	return time.Duration(*seconds) * time.Second, nil
}

func validateTerminationBudgets(budgets terminationBudgets, platformMargin time.Duration) error {
	requiredGrace := budgets.stopTimeout + platformMargin
	var errs []error
	if budgets.kubernetesGrace < requiredGrace {
		errs = append(errs, fmt.Errorf(
			"kubernetes termination grace %s is less than required %s (stop_timeout=%s platform_margin=%s)",
			budgets.kubernetesGrace,
			requiredGrace,
			budgets.stopTimeout,
			platformMargin,
		))
	}
	if budgets.helmGrace < requiredGrace {
		errs = append(errs, fmt.Errorf(
			"helm termination grace %s is less than required %s (stop_timeout=%s platform_margin=%s)",
			budgets.helmGrace,
			requiredGrace,
			budgets.stopTimeout,
			platformMargin,
		))
	}
	if budgets.kubernetesGrace != budgets.helmGrace {
		errs = append(errs, fmt.Errorf(
			"termination grace drift: kubernetes=%s helm=%s",
			budgets.kubernetesGrace,
			budgets.helmGrace,
		))
	}
	return errors.Join(errs...)
}
