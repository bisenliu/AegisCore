package containers

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	// 注册测试容器使用的 pgx database/sql driver。
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/common/runtime/resources"
)

const (
	// EnvTestContainers 控制是否启用真实 Docker-backed integration containers。
	EnvTestContainers = "AEGISCORE_TEST_CONTAINERS"

	DefaultPostgresImage           = "postgres:15-alpine"
	DefaultPostgresDatabase        = "aegiscore_test"
	DefaultPostgresUsername        = "aegiscore"
	DefaultPostgresPassword        = "secret"
	DefaultStartupTimeout          = 90 * time.Second
	defaultPostgresPort            = "5432/tcp"
	defaultDockerPortProbeInterval = time.Millisecond * 100
)

type PostgresOptions struct {
	Image          string
	Database       string
	Username       string
	Password       string
	StartupTimeout time.Duration
}

type PostgresContainer struct {
	ContainerID string
	Host        string
	Port        int
	Database    string
	Username    string
	Password    string
	DSN         string
}

func StartPostgres(ctx context.Context, t testing.TB, opts PostgresOptions) *PostgresContainer {
	t.Helper()
	requireContainersEnabled(t)

	opts = normalizePostgresOptions(opts)
	startCtx, cancel := context.WithTimeout(ctx, opts.StartupTimeout)
	defer cancel()

	containerID := dockerRun(startCtx, t,
		"-d",
		"--rm",
		"-e", "POSTGRES_DB="+opts.Database,
		"-e", "POSTGRES_USER="+opts.Username,
		"-e", "POSTGRES_PASSWORD="+opts.Password,
		"-p", "127.0.0.1::5432",
		opts.Image,
	)
	t.Cleanup(func() { dockerStop(context.Background(), t, containerID) })

	host, port := dockerMappedPort(startCtx, t, containerID, defaultPostgresPort)
	pg := &PostgresContainer{
		ContainerID: containerID,
		Host:        host,
		Port:        port,
		Database:    opts.Database,
		Username:    opts.Username,
		Password:    opts.Password,
	}
	pg.DSN = datastore.PostgresDSN(pg.Config())

	waitForPostgres(startCtx, t, pg)
	return pg
}

func (p PostgresContainer) Config() resources.PostgresConfig {
	return resources.PostgresConfig{
		Host:     p.Host,
		Port:     p.Port,
		Username: p.Username,
		Password: p.Password,
		DBName:   p.Database,
		SSLMode:  resources.DefaultPostgresSSLMode,
		Pool: resources.PostgresPoolConfig{
			MaxOpenConns:    2,
			MaxIdleConns:    1,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: time.Minute,
		},
	}
}

func ContainersEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(EnvTestContainers)))
	return value == "1" || value == "true" || value == "yes"
}

func normalizePostgresOptions(opts PostgresOptions) PostgresOptions {
	if opts.Image == "" {
		opts.Image = DefaultPostgresImage
	}
	if opts.Database == "" {
		opts.Database = DefaultPostgresDatabase
	}
	if opts.Username == "" {
		opts.Username = DefaultPostgresUsername
	}
	if opts.Password == "" {
		opts.Password = DefaultPostgresPassword
	}
	if opts.StartupTimeout <= 0 {
		opts.StartupTimeout = DefaultStartupTimeout
	}
	return opts
}

func requireContainersEnabled(t testing.TB) {
	t.Helper()
	if !ContainersEnabled() {
		t.Skipf("set %s=1 to enable Docker-backed integration containers", EnvTestContainers)
	}
}

func dockerRun(ctx context.Context, t testing.TB, args ...string) string {
	t.Helper()
	out, stderr, err := dockerOutput(ctx, append([]string{"run"}, args...)...)
	if err != nil {
		t.Fatalf("start Docker container: %v: %s", err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(out)
}

func dockerStop(ctx context.Context, t testing.TB, containerID string) {
	t.Helper()
	if containerID == "" {
		return
	}
	stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, stderr, err := dockerOutput(stopCtx, "stop", "-t", "1", containerID)
	if err != nil {
		t.Errorf("stop Docker container %s: %v: %s", containerID, err, strings.TrimSpace(stderr+out))
	}
}

func dockerMappedPort(ctx context.Context, t testing.TB, containerID, containerPort string) (string, int) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(DefaultStartupTimeout)
	}

	var lastErr error
	for time.Now().Before(deadline) {
		out, stderr, err := dockerOutput(ctx, "port", containerID, containerPort)
		if err == nil {
			host, port, parseErr := parseDockerPort(out)
			if parseErr == nil {
				return host, port
			}
			lastErr = parseErr
		} else {
			lastErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr+out))
		}
		time.Sleep(defaultDockerPortProbeInterval)
	}
	t.Fatalf("resolve mapped Docker port %s for %s: %v", containerPort, containerID, lastErr)
	return "", 0
}

func parseDockerPort(output string) (string, int, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		host, portValue, err := net.SplitHostPort(line)
		if err != nil {
			return "", 0, err
		}
		port, err := strconv.Atoi(portValue)
		if err != nil {
			return "", 0, err
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		return host, port, nil
	}
	return "", 0, fmt.Errorf("empty Docker port output")
}

func waitForPostgres(ctx context.Context, t testing.TB, pg *PostgresContainer) {
	t.Helper()
	db, err := sql.Open("pgx", pg.DSN)
	if err != nil {
		t.Fatalf("open PostgreSQL test container: %v", err)
	}
	defer func() { _ = db.Close() }()

	waitFor(ctx, t, "PostgreSQL ping", func(pingCtx context.Context) error {
		return db.PingContext(pingCtx)
	})
}

func waitFor(ctx context.Context, t testing.TB, label string, check func(context.Context) error) {
	t.Helper()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		checkCtx, cancel := context.WithTimeout(ctx, time.Second)
		err := check(checkCtx)
		cancel()
		if err == nil {
			return
		}
		lastErr = err

		select {
		case <-ctx.Done():
			t.Fatalf("%s did not become ready: %v: %v", label, ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func dockerOutput(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return string(out), stderr.String(), err
}
