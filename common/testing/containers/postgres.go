package containers

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
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
	// DefaultPostgresImage 是 PostgreSQL 测试容器的默认镜像。
	DefaultPostgresImage = "postgres:15-alpine"
	// DefaultPostgresDatabase 是 PostgreSQL 测试容器的默认数据库名。
	DefaultPostgresDatabase = "aegiscore_test"
	// DefaultPostgresUsername 是 PostgreSQL 测试容器的默认用户名。
	DefaultPostgresUsername = "aegiscore"
	// DefaultPostgresPassword 是 PostgreSQL 测试容器的默认密码，仅用于隔离的本地测试容器。
	DefaultPostgresPassword = "secret"
	// DefaultStartupTimeout 是测试容器启动和就绪探测的默认总超时。
	DefaultStartupTimeout = 90 * time.Second

	defaultPostgresPort            = "5432/tcp"
	defaultDockerPortProbeInterval = time.Millisecond * 100
)

var testContainersEnabled = flag.Bool("aegiscore.testcontainers", false, "enable Docker-backed integration containers")

// PostgresOptions 配置 PostgreSQL 测试容器；零值字段由 StartPostgres 补为测试默认值。
type PostgresOptions struct {
	Image          string
	Database       string
	Username       string
	Password       string
	StartupTimeout time.Duration
}

// PostgresContainer 描述已启动测试容器的连接信息。
// 容器生命周期由 StartPostgres 注册到 testing.TB.Cleanup，无需调用方手动停止。
type PostgresContainer struct {
	ContainerID string
	Host        string
	Port        int
	Database    string
	Username    string
	Password    string
	DSN         string
}

// StartPostgres 启动 PostgreSQL 测试容器并等待其可接受连接。
// 未启用 -aegiscore.testcontainers 时跳过测试；启动或就绪失败时通过 testing.TB 终止当前测试。
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

// Config 返回与测试容器连接信息匹配的运行时 PostgreSQL 配置。
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

// ContainersEnabled 报告当前测试进程是否显式启用了 Docker 测试容器。
func ContainersEnabled() bool {
	return testContainersEnabled != nil && *testContainersEnabled
}

// normalizePostgresOptions 补上测试数据库没填的库名、用户名和镜像。
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

// requireContainersEnabled 没打开容器测试时跳过当前测试，避免本地没装 Docker 时误失败。
func requireContainersEnabled(t testing.TB) {
	t.Helper()
	if !ContainersEnabled() {
		t.Skip("pass -args -aegiscore.testcontainers to enable Docker-backed integration containers")
	}
}

// dockerRun 启动测试容器，失败时让测试停下来并保留命令输出。
func dockerRun(ctx context.Context, t testing.TB, args ...string) string {
	t.Helper()
	out, stderr, err := dockerOutput(ctx, append([]string{"run"}, args...)...)
	if err != nil {
		t.Fatalf("start Docker container: %v: %s", err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(out)
}

// dockerStop 停掉并删除测试容器，清理失败也要让测试看到原因。
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

// dockerMappedPort 查出容器端口映射到本机的地址，供测试连接使用。
func dockerMappedPort(ctx context.Context, t testing.TB, containerID, containerPort string) (string, int) {
	t.Helper()
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultStartupTimeout)
		defer cancel()
	}
	ticker := time.NewTicker(defaultDockerPortProbeInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		// docker run 返回后端口映射可能尚未可查询，因此在同一启动 deadline 内轮询。
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

		select {
		case <-ctx.Done():
			t.Fatalf("resolve mapped Docker port %s for %s: %v: %v", containerPort, containerID, ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

// parseDockerPort 从容器端口映射输出中提取数字端口号；如果提取出的内容不是合法的数字则返回错误。
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

// waitForPostgres 反复尝试连接测试数据库容器，直到数据库准备完毕可以接收请求；若超过等待时限仍连不上则终止测试。
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

// waitFor 定时轮询检查某项服务是否准备就绪，直到检查通过或超时退出。
func waitFor(ctx context.Context, t testing.TB, label string, check func(context.Context) error) {
	t.Helper()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		// 单次探测使用短 timeout，避免某次阻塞耗尽整个容器启动期限。
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

// dockerOutput 执行 Docker 命令并返回标准输出和错误输出。
func dockerOutput(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return string(out), stderr.String(), err
}
