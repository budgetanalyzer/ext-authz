package main

import "os"

// Config holds all ext-authz service configuration, loaded from environment variables.
type Config struct {
	RedisAddr         string
	RedisUsername     string
	RedisPassword     string
	RedisTLS          bool
	RedisCACert       string
	HTTPPort          string
	HealthPort        string
	SessionKeyPrefix  string
	SessionCookieName string
	LogLevel          string
	LogFormat         string
}

func LoadConfig() Config {
	return Config{
		RedisAddr:         envOrDefault("REDIS_ADDR", "redis.infrastructure:6379"),
		RedisUsername:     envOrDefault("REDIS_USERNAME", ""),
		RedisPassword:     envOrDefault("REDIS_EXT_AUTHZ_PASSWORD", ""),
		RedisTLS:          envOrDefault("REDIS_TLS", "false") == "true",
		RedisCACert:       envOrDefault("REDIS_CA_CERT", ""),
		HTTPPort:          envOrDefault("HTTP_PORT", "9002"),
		HealthPort:        envOrDefault("HEALTH_PORT", "8090"),
		SessionKeyPrefix:  envOrDefault("SESSION_KEY_PREFIX", "session:"),
		SessionCookieName: envOrDefault("SESSION_COOKIE_NAME", "BA_SESSION"),
		LogLevel:          envOrDefault("LOG_LEVEL", "info"),
		LogFormat:         envOrDefault("LOG_FORMAT", "json"),
	}
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
