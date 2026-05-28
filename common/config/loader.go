package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

const envPrefix = "AEGISCORE"

func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigType("yaml")
	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath("./configs")
		v.AddConfigPath("../configs")
	}

	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	if err := bindEnvKeys(v); err != nil {
		return nil, err
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := validateRequiredKeys(v); err != nil {
		return nil, err
	}
	if err := validateStruct(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateRequiredKeys(v *viper.Viper) error {
	for _, key := range explicitRequiredKeys {
		if !v.IsSet(key) {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}

func validateStruct(cfg Config) error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("mapstructure"), ",", 2)[0]
		if name == "" || name == "-" {
			return field.Name
		}
		return name
	})
	if err := validate.Struct(cfg); err != nil {
		return formatValidationError(err)
	}
	return nil
}

func formatValidationError(err error) error {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok || len(validationErrors) == 0 {
		return fmt.Errorf("validate config: %w", err)
	}
	errField := validationErrors[0]
	field := configPath(errField.Namespace())
	switch errField.Tag() {
	case "required":
		return fmt.Errorf("%s is required", field)
	case "min":
		return fmt.Errorf("%s must be at least %s", field, errField.Param())
	case "max":
		return fmt.Errorf("%s must be at most %s", field, errField.Param())
	case "gt":
		return fmt.Errorf("%s must be greater than %s", field, errField.Param())
	default:
		return fmt.Errorf("%s failed validation %s", field, errField.Tag())
	}
}

func configPath(namespace string) string {
	path := strings.ToLower(namespace)
	return strings.TrimPrefix(path, "config.")
}

var explicitRequiredKeys = []string{
	"app.name",
	"app.environment",
	"http.host",
	"http.port",
	"http.read_timeout",
	"http.write_timeout",
	"http.idle_timeout",
	"http.shutdown_timeout",
	"log.level",
	"log.format",
	"redis.addr",
	"redis.db",
	"redis.dial_timeout",
	"redis.read_timeout",
	"redis.write_timeout",
	"database.postgres.host",
	"database.postgres.port",
	"database.postgres.username",
	"database.postgres.user_db_name",
	"database.postgres.pay_db_name",
	"database.postgres.common_db_name",
	"database.postgres.driver",
	"database.postgres.sslmode",
	"database.postgres.max_open_conns",
	"database.postgres.max_idle_conns",
	"database.postgres.conn_max_lifetime",
	"database.postgres.conn_max_idle_time",
	"database.postgres.ping_timeout",
}

var envKeys = append([]string{
	"http.trusted_proxies",
	"redis.username",
	"redis.password",
	"database.postgres.password",
}, explicitRequiredKeys...)

func bindEnvKeys(v *viper.Viper) error {
	for _, key := range envKeys {
		if err := v.BindEnv(key); err != nil {
			return fmt.Errorf("bind env %s: %w", key, err)
		}
	}
	return nil
}
