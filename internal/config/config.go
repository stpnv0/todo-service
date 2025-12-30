package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server  HTTPServer
	Logging LoggingConfig
}

type HTTPServer struct {
	Port        string        `env:"HTTP_PORT"`
	Host        string        `env:"HTTP_HOST"`
	Timeout     time.Duration `env:"HTTP_TIMEOUT"`
	IdleTimeout time.Duration `env:"HTTP_IDLE_TIMEOUT"`
}

type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"`
	Format string `env:"LOG_FORMAT"`
}

func (h *HTTPServer) GetAddr() string {
	return net.JoinHostPort(h.Host, h.Port)
}

func MustLoad() *Config {
	cfg, err := load()
	if err != nil {
		panic(err)
	}

	return cfg
}

func load() (*Config, error) {
	cfg := &Config{
		Server: HTTPServer{
			Port:        getEnv("HTTP_PORT", "8080"),
			Host:        getEnv("HTTP_HOST", "0.0.0.0"),
			Timeout:     parseDurationEnv("HTTP_TIMEOUT", 15*time.Second),
			IdleTimeout: parseDurationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getEnv возвращает значение из env или default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseDurationEnv возвращает время из env или default value
// поддеживаются разные форматы времени: "15s", "1m30s", "15" (секунд)
func parseDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	// duration строка
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}

	// число
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}

	return defaultValue
}

func (c *Config) Validate() error {
	port, err := strconv.Atoi(c.Server.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %s (must be 1-65535)", c.Server.Port)
	}

	if c.Server.Host != "localhost" && c.Server.Host != "0.0.0.0" {
		if ip := net.ParseIP(c.Server.Host); ip == nil {
			return fmt.Errorf("invalid host: %s", c.Server.Host)
		}
	}

	if c.Server.Timeout < time.Second {
		return fmt.Errorf("timeout too small: %v (minimum 1s)", c.Server.Timeout)
	}
	if c.Server.Timeout > 5*time.Minute {
		return fmt.Errorf("timeout too large: %v (maximum 5m)", c.Server.Timeout)
	}

	if c.Server.IdleTimeout < time.Second {
		return fmt.Errorf("idle timeout too small: %v (minimum 1s)", c.Server.IdleTimeout)
	}

	validLevels := map[string]struct{}{
		"debug": {},
		"info":  {},
		"warn":  {},
		"error": {},
	}
	level := strings.ToLower(c.Logging.Level)
	if _, ok := validLevels[level]; !ok {
		return fmt.Errorf("invalid log level: %s (allowed: debug, info, warn, error)", c.Logging.Level)
	}

	validFormats := map[string]struct{}{
		"json": {},
		"text": {},
	}
	format := strings.ToLower(c.Logging.Format)
	if _, ok := validFormats[format]; !ok {
		return fmt.Errorf("invalid log format: %s (allowed: json, text)", c.Logging.Format)
	}

	return nil
}
