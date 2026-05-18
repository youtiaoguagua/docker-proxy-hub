package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type Config struct {
	Addr                string        `mapstructure:"addr"`
	DBPath              string        `mapstructure:"db_path"`
	JWTSecret           string        `mapstructure:"jwt_secret"`
	JWTTTL              time.Duration `mapstructure:"jwt_ttl"`
	CookieName          string        `mapstructure:"cookie_name"`
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"`
	LogLevel            string        `mapstructure:"log_level"`
	LogFormat           string        `mapstructure:"log_format"`
}

func Load() (Config, error) {
	v := viper.New()

	setDefaults(v)

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/docker-proxy-hub")

	v.SetEnvPrefix("DPH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := bindEnvs(v); err != nil {
		return Config{}, err
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.StringToTimeDurationHookFunc())); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("addr", ":8080")
	v.SetDefault("db_path", "./docker-proxy-hub.db")
	v.SetDefault("jwt_ttl", "24h")
	v.SetDefault("cookie_name", "dph_token")
	v.SetDefault("health_check_interval", "30s")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")
}

func bindEnvs(v *viper.Viper) error {
	keys := []string{
		"addr",
		"db_path",
		"jwt_secret",
		"jwt_ttl",
		"cookie_name",
		"health_check_interval",
		"log_level",
		"log_format",
	}

	for _, key := range keys {
		if err := v.BindEnv(key); err != nil {
			return fmt.Errorf("bind env %q: %w", key, err)
		}
	}

	return nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.JWTSecret) == "" {
		return fmt.Errorf("jwt_secret is required")
	}

	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("addr must not be empty")
	}

	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log_level %q", c.LogLevel)
	}

	switch c.LogFormat {
	case "json", "text":
	default:
		return fmt.Errorf("invalid log_format %q", c.LogFormat)
	}

	return nil
}
