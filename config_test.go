package main

import "testing"

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_USERNAME", "")
	t.Setenv("REDIS_EXT_AUTHZ_PASSWORD", "")
	t.Setenv("REDIS_TLS", "")
	t.Setenv("REDIS_CA_CERT", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("HEALTH_PORT", "")
	t.Setenv("SESSION_KEY_PREFIX", "")
	t.Setenv("SESSION_COOKIE_NAME", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")

	cfg := LoadConfig()

	if cfg.SessionKeyPrefix != "session:" {
		t.Fatalf("SessionKeyPrefix = %q, want %q", cfg.SessionKeyPrefix, "session:")
	}
	if cfg.SessionCookieName != "BA_SESSION" {
		t.Fatalf("SessionCookieName = %q, want %q", cfg.SessionCookieName, "BA_SESSION")
	}
	if cfg.HTTPPort != "9002" {
		t.Fatalf("HTTPPort = %q, want %q", cfg.HTTPPort, "9002")
	}
	if cfg.HealthPort != "8090" {
		t.Fatalf("HealthPort = %q, want %q", cfg.HealthPort, "8090")
	}
}

func TestLoadConfigSessionOverrides(t *testing.T) {
	t.Setenv("SESSION_KEY_PREFIX", "custom-session:")
	t.Setenv("SESSION_COOKIE_NAME", "BUDGET_SESSION")

	cfg := LoadConfig()

	if cfg.SessionKeyPrefix != "custom-session:" {
		t.Fatalf("SessionKeyPrefix = %q, want %q", cfg.SessionKeyPrefix, "custom-session:")
	}
	if cfg.SessionCookieName != "BUDGET_SESSION" {
		t.Fatalf("SessionCookieName = %q, want %q", cfg.SessionCookieName, "BUDGET_SESSION")
	}
}
