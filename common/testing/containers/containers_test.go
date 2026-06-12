package containers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestStartPostgresIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	pg := StartPostgres(ctx, t, PostgresOptions{})
	db, err := sql.Open("pgx", pg.DSN)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	cfg := pg.Config()
	if cfg.Driver != "pgx" || cfg.Host == "" || cfg.Port == 0 || cfg.DBName != DefaultPostgresDatabase {
		t.Fatalf("Postgres config = %#v", cfg)
	}
}

func TestStartRedisIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	redisContainer := StartRedis(ctx, t, RedisOptions{})
	client := redis.NewClient(redisContainer.Options())
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	cfg := redisContainer.Config()
	if cfg.Addr == "" || cfg.DB != 0 || cfg.PingTimeout <= 0 {
		t.Fatalf("Redis config = %#v", cfg)
	}
}
