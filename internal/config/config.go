package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Addr                string
	DBPath              string
	JWTSecret           string
	JWTTTL              time.Duration
	CookieName          string
	HealthCheckInterval time.Duration
	LogLevel            string
	LogFormat           string
}

func Load() (Config, error) {
	v := viper.New()

	v.SetDefault("addr", ":8080")
	v.SetDefault("db_path", "./docker-proxy-hub.db")
	v.SetDefault("jwt_secret", "")
	v.SetDefault("jwt_ttl", "24h")
	v.SetDefault("cookie_name", "dph_token")
	v.SetDefault("health_check_interval", "30s")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/docker-proxy-hub")

	v.SetEnvPrefix("DPH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	jwtTTL, err := time.ParseDuration(v.GetString("jwt_ttl"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid jwt_ttl: %w", err)
	}

	healthCheckInterval, err := time.ParseDuration(v.GetString("health_check_interval"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid health_check_interval: %w", err)
	}

	return Config{
		Addr:                v.GetString("addr"),
		DBPath:              v.GetString("db_path"),
		JWTSecret:           v.GetString("jwt_secret"),
		JWTTTL:              jwtTTL,
		CookieName:          v.GetString("cookie_name"),
		HealthCheckInterval: healthCheckInterval,
		LogLevel:            v.GetString("log_level"),
		LogFormat:           v.GetString("log_format"),
	}, nil
}
