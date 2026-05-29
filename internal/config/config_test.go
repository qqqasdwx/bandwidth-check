package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsInvalidInteger(t *testing.T) {
	t.Setenv("ROUTER_URL", "http://router.example")
	t.Setenv("ROUTER_PASSWORD", "password")
	t.Setenv("KUMA_PUSH_URL", "http://kuma.example/api/push/token")
	t.Setenv("CHECK_INTERVAL_SECONDS", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error, want invalid integer error")
	}
	if !strings.Contains(err.Error(), "CHECK_INTERVAL_SECONDS 必须是整数") {
		t.Fatalf("Load error = %q, want CHECK_INTERVAL_SECONDS integer error", err)
	}
}

func TestLoadDefaultsRuntimeOptions(t *testing.T) {
	t.Setenv("ROUTER_URL", "http://router.example")
	t.Setenv("ROUTER_PASSWORD", "password")
	t.Setenv("KUMA_PUSH_URL", "http://kuma.example/api/push/token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.RouterRetries != 1 {
		t.Fatalf("RouterRetries = %d, want 1", cfg.RouterRetries)
	}
	if cfg.RouterRetryDelay.Milliseconds() != 300 {
		t.Fatalf("RouterRetryDelay = %s, want 300ms", cfg.RouterRetryDelay)
	}
}
