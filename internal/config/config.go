package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RouterURL     string
	RouterUser    string
	RouterPass    string
	KumaPushURL   string
	WANPortAlias  string
	MinSpeedMbps  int
	CheckInterval time.Duration
	HTTPTimeout   time.Duration
	RunOnce       bool
}

func Load() (Config, error) {
	cfg := Config{
		RouterURL:     strings.TrimSpace(os.Getenv("ROUTER_URL")),
		RouterUser:    strings.TrimSpace(os.Getenv("ROUTER_USERNAME")),
		RouterPass:    os.Getenv("ROUTER_PASSWORD"),
		KumaPushURL:   strings.TrimSpace(os.Getenv("KUMA_PUSH_URL")),
		WANPortAlias:  getEnv("WAN_PORT_ALIAS", "ETH_WAN"),
		MinSpeedMbps:  getEnvInt("MIN_SPEED_MBPS", 1000),
		CheckInterval: time.Duration(getEnvInt("CHECK_INTERVAL_SECONDS", 60)) * time.Second,
		HTTPTimeout:   time.Duration(getEnvInt("HTTP_TIMEOUT_SECONDS", 10)) * time.Second,
		RunOnce:       getEnvBool("RUN_ONCE", false),
	}

	if cfg.RouterURL == "" {
		return Config{}, fmt.Errorf("ROUTER_URL is required")
	}
	if cfg.RouterPass == "" {
		return Config{}, fmt.Errorf("ROUTER_PASSWORD is required")
	}
	if cfg.KumaPushURL == "" {
		return Config{}, fmt.Errorf("KUMA_PUSH_URL is required")
	}
	if err := validateHTTPURL("KUMA_PUSH_URL", cfg.KumaPushURL); err != nil {
		return Config{}, err
	}
	if cfg.MinSpeedMbps <= 0 {
		return Config{}, fmt.Errorf("MIN_SPEED_MBPS must be positive")
	}
	if cfg.CheckInterval <= 0 {
		return Config{}, fmt.Errorf("CHECK_INTERVAL_SECONDS must be positive")
	}
	if cfg.HTTPTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP_TIMEOUT_SECONDS must be positive")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func validateHTTPURL(key, value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s is invalid: %w", key, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", key)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s must include a host", key)
	}
	return nil
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
