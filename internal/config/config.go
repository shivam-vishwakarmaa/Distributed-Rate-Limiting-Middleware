package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Redis          RedisConfig            `mapstructure:"redis"`
	CircuitBreaker CircuitBreakerConfig   `mapstructure:"circuit_breaker"`
	Routes         []RouteConfig          `mapstructure:"routes"`
}

type RedisConfig struct {
	Addr             string `mapstructure:"addr"`
	DialTimeoutMs    int    `mapstructure:"dial_timeout_ms"`
	CommandTimeoutMs int    `mapstructure:"command_timeout_ms"`
}

type CircuitBreakerConfig struct {
	FailureThreshold    int `mapstructure:"failure_threshold"`
	OpenDurationSeconds int `mapstructure:"open_duration_seconds"`
}

type RouteConfig struct {
	PathPrefix      string `mapstructure:"path_prefix"`
	Algorithm       string `mapstructure:"algorithm"`
	Limit           int    `mapstructure:"limit"`
	WindowSeconds   int    `mapstructure:"window_seconds"`
	Capacity        int    `mapstructure:"capacity"`
	RefillPerSecond int    `mapstructure:"refill_per_second"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
