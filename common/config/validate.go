package config

import (
	"fmt"
	"time"
)

func (c Config) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}
	if c.App.Environment == "" {
		return fmt.Errorf("app.environment is required")
	}
	if c.HTTP.Host == "" {
		return fmt.Errorf("http.host is required")
	}
	if err := validatePort("http.port", c.HTTP.Port); err != nil {
		return err
	}
	if err := validatePositiveDuration("http.read_timeout", c.HTTP.ReadTimeout); err != nil {
		return err
	}
	if err := validatePositiveDuration("http.write_timeout", c.HTTP.WriteTimeout); err != nil {
		return err
	}
	if err := validatePositiveDuration("http.idle_timeout", c.HTTP.IdleTimeout); err != nil {
		return err
	}
	if err := validatePositiveDuration("http.shutdown_timeout", c.HTTP.ShutdownTimeout); err != nil {
		return err
	}
	if c.Log.Level == "" {
		return fmt.Errorf("log.level is required")
	}
	if c.Log.Format == "" {
		return fmt.Errorf("log.format is required")
	}
	if len(c.Redis) == 0 {
		return fmt.Errorf("redis must declare at least one instance")
	}
	for name, redisCfg := range c.Redis {
		if err := redisCfg.validate("redis." + name); err != nil {
			return err
		}
	}
	if len(c.Postgre) == 0 {
		return fmt.Errorf("postgre must declare at least one instance")
	}
	for name, pg := range c.Postgre {
		if err := pg.validate("postgre." + name); err != nil {
			return err
		}
	}
	return nil
}

func (r RedisConfig) validate(path string) error {
	if r.Addr == "" {
		return fmt.Errorf("%s.addr is required", path)
	}
	if r.DB < 0 {
		return fmt.Errorf("%s.db must be >= 0", path)
	}
	if err := validatePositiveDuration(path+".dial_timeout", r.DialTimeout); err != nil {
		return err
	}
	if err := validatePositiveDuration(path+".read_timeout", r.ReadTimeout); err != nil {
		return err
	}
	return validatePositiveDuration(path+".write_timeout", r.WriteTimeout)
}

func (p PostgresConfig) validate(path string) error {
	if p.Host == "" {
		return fmt.Errorf("%s.host is required", path)
	}
	if err := validatePort(path+".port", p.Port); err != nil {
		return err
	}
	if p.Username == "" {
		return fmt.Errorf("%s.username is required", path)
	}
	if p.DBName == "" {
		return fmt.Errorf("%s.db_name is required", path)
	}
	if p.Driver == "" {
		return fmt.Errorf("%s.driver is required", path)
	}
	if p.MaxOpenConns <= 0 {
		return fmt.Errorf("%s.max_open_conns must be > 0", path)
	}
	if p.MaxIdleConns < 0 {
		return fmt.Errorf("%s.max_idle_conns must be >= 0", path)
	}
	if p.MaxIdleConns > p.MaxOpenConns {
		return fmt.Errorf("%s.max_idle_conns must be <= max_open_conns", path)
	}
	if err := validatePositiveDuration(path+".conn_max_lifetime", p.ConnMaxLifetime); err != nil {
		return err
	}
	if err := validatePositiveDuration(path+".conn_max_idle_time", p.ConnMaxIdleTime); err != nil {
		return err
	}
	return validatePositiveDuration(path+".ping_timeout", p.PingTimeout)
}

func validatePort(path string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", path)
	}
	return nil
}

func validatePositiveDuration(path string, d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("%s must be > 0", path)
	}
	return nil
}
