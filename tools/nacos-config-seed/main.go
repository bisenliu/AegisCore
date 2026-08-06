package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	exitOK    = 0
	exitError = 1
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var options seedOptions
	var dataIDsCSV string
	var timeoutText string

	flags := flag.NewFlagSet("nacos-config-seed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.Addr, "addr", envOrDefault("AEGISCORE_NACOS_ADDR", "nacos:8848"), "Nacos server address")
	flags.StringVar(&options.Namespace, "namespace", envOrDefault("AEGISCORE_NACOS_NAMESPACE", "loca"), "Nacos namespace ID")
	flags.StringVar(&options.Group, "group", envOrDefault("AEGISCORE_NACOS_GROUP", "AEGISCORE"), "Nacos config group")
	flags.StringVar(&options.ConfigDir, "config-dir", "/nacos/init", "directory containing YAML documents")
	flags.StringVar(&dataIDsCSV, "data-ids", defaultDataIDs(), "comma-separated Nacos data IDs")
	flags.StringVar(&timeoutText, "timeout", envOrDefault("AEGISCORE_NACOS_TIMEOUT", "10s"), "overall request timeout")
	flags.StringVar(&options.Username, "username", strings.TrimSpace(os.Getenv("AEGISCORE_NACOS_USERNAME")), "Nacos username")
	flags.StringVar(&options.Password, "password", secretEnv("AEGISCORE_NACOS_PASSWORD"), "Nacos password")
	if err := flags.Parse(args); err != nil {
		return exitError
	}

	var err error
	options.Timeout, err = time.ParseDuration(strings.TrimSpace(timeoutText))
	if err != nil || options.Timeout <= 0 {
		failf(stderr, "timeout must be a positive duration")
		return exitError
	}
	options.DataIDs, err = parseDataIDs(dataIDsCSV)
	if err != nil {
		failf(stderr, "%v", err)
		return exitError
	}
	if err := options.Validate(); err != nil {
		failf(stderr, "%v", err)
		return exitError
	}

	documents := make(map[string][]byte, len(options.DataIDs))
	// 在发起网络请求前完整读取所有文档，避免中途才发现本地文件不可读而造成部分发布。
	for _, dataID := range options.DataIDs {
		content, readErr := os.ReadFile(filepath.Join(options.ConfigDir, dataID))
		if readErr != nil {
			failf(stderr, "read config document %s: %v", dataID, readErr)
			return exitError
		}
		if len(strings.TrimSpace(string(content))) == 0 {
			failf(stderr, "read config document %s: content is empty", dataID)
			return exitError
		}
		documents[dataID] = content
	}

	client, err := newAdminClient(options)
	if err != nil {
		failf(stderr, "%v", err)
		return exitError
	}
	if err := client.Seed(ctx, options, documents); err != nil {
		failf(stderr, "%v", err)
		return exitError
	}
	if _, err := fmt.Fprintf(
		stdout,
		"seeded Nacos namespace=%s group=%s data_ids=%s\n",
		options.Namespace,
		options.Group,
		strings.Join(options.DataIDs, ","),
	); err != nil {
		failf(stderr, "write success output: %v", err)
		return exitError
	}
	return exitOK
}

func defaultDataIDs() string {
	service := envOrDefault("AEGISCORE_SERVICE", "user-service")
	return "base.yaml,resources.yaml," + service + ".yaml"
}

func parseDataIDs(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	dataIDs := make([]string, 0, len(parts))
	for _, part := range parts {
		dataID := strings.TrimSpace(part)
		if dataID == "" {
			return nil, fmt.Errorf("data-ids contains an empty value")
		}
		// data-id 只作为配置名使用，不允许通过路径片段逃逸 config-dir。
		if filepath.Base(dataID) != dataID {
			return nil, fmt.Errorf("data-id %q must be a file name", dataID)
		}
		dataIDs = append(dataIDs, dataID)
	}
	return dataIDs, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func secretEnv(name string) string {
	value := os.Getenv(name)
	if strings.TrimSpace(value) == "" {
		return ""
	}
	// 凭据的首尾空白可能有意义，只用 TrimSpace 判断是否缺失，不改写原值。
	return value
}

func failf(stderr io.Writer, format string, args ...any) {
	// 调用方已经确定返回失败；诊断 writer 不可用时仍保持原非零退出状态。
	_, _ = fmt.Fprintf(stderr, format+"\n", args...)
}
