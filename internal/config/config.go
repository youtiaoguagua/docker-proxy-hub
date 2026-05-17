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

func Load() Config {
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

	_ = v.BindEnv("addr")
	_ = v.BindEnv("db_path")
	_ = v.BindEnv("jwt_secret")
	_ = v.BindEnv("jwt_ttl")
	_ = v.BindEnv("cookie_name")
	_ = v.BindEnv("health_check_interval")
	_ = v.BindEnv("log_level")
	_ = v.BindEnv("log_format")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(fmt.Sprintf("failed to read config: %v", err))
		}
	}

	jwtTTL, err := time.ParseDuration(v.GetString("jwt_ttl"))
	if err != nil {
		jwtTTL = 24 * time.Hour
	}

	healthCheckInterval, err := time.ParseDuration(v.GetString("health_check_interval"))
	if err != nil {
		healthCheckInterval = 30 * time.Second
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
	}
}