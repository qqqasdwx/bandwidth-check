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
	RouterURL        string
	RouterUser       string
	RouterPass       string
	KumaPushURL      string
	WANPortAlias     string
	MinSpeedMbps     int
	CheckInterval    time.Duration
	HTTPTimeout      time.Duration
	LogLevel         string
	RouterRetries    int
	RouterRetryDelay time.Duration
	RunOnce          bool
}

func Load() (Config, error) {
	minSpeedMbps, err := getEnvInt("MIN_SPEED_MBPS", 1000)
	if err != nil {
		return Config{}, err
	}
	checkIntervalSeconds, err := getEnvInt("CHECK_INTERVAL_SECONDS", 60)
	if err != nil {
		return Config{}, err
	}
	httpTimeoutSeconds, err := getEnvInt("HTTP_TIMEOUT_SECONDS", 10)
	if err != nil {
		return Config{}, err
	}
	routerRetries, err := getEnvInt("ROUTER_RETRY_COUNT", 1)
	if err != nil {
		return Config{}, err
	}
	routerRetryDelayMS, err := getEnvInt("ROUTER_RETRY_DELAY_MS", 300)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		RouterURL:        strings.TrimSpace(os.Getenv("ROUTER_URL")),
		RouterUser:       strings.TrimSpace(os.Getenv("ROUTER_USERNAME")),
		RouterPass:       os.Getenv("ROUTER_PASSWORD"),
		KumaPushURL:      strings.TrimSpace(os.Getenv("KUMA_PUSH_URL")),
		WANPortAlias:     getEnv("WAN_PORT_ALIAS", "ETH_WAN"),
		MinSpeedMbps:     minSpeedMbps,
		CheckInterval:    time.Duration(checkIntervalSeconds) * time.Second,
		HTTPTimeout:      time.Duration(httpTimeoutSeconds) * time.Second,
		LogLevel:         strings.ToLower(getEnv("LOG_LEVEL", "info")),
		RouterRetries:    routerRetries,
		RouterRetryDelay: time.Duration(routerRetryDelayMS) * time.Millisecond,
		RunOnce:          getEnvBool("RUN_ONCE", false),
	}

	if cfg.RouterURL == "" {
		return Config{}, fmt.Errorf("ROUTER_URL 必填")
	}
	if cfg.RouterPass == "" {
		return Config{}, fmt.Errorf("ROUTER_PASSWORD 必填")
	}
	if cfg.KumaPushURL == "" {
		return Config{}, fmt.Errorf("KUMA_PUSH_URL 必填")
	}
	if err := validateHTTPURL("KUMA_PUSH_URL", cfg.KumaPushURL); err != nil {
		return Config{}, err
	}
	if cfg.MinSpeedMbps <= 0 {
		return Config{}, fmt.Errorf("MIN_SPEED_MBPS 必须是正数")
	}
	if cfg.CheckInterval <= 0 {
		return Config{}, fmt.Errorf("CHECK_INTERVAL_SECONDS 必须是正数")
	}
	if cfg.HTTPTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP_TIMEOUT_SECONDS 必须是正数")
	}
	if cfg.LogLevel != "info" && cfg.LogLevel != "debug" {
		return Config{}, fmt.Errorf("LOG_LEVEL 只能是 info 或 debug")
	}
	if cfg.RouterRetries < 0 {
		return Config{}, fmt.Errorf("ROUTER_RETRY_COUNT 不能是负数")
	}
	if cfg.RouterRetryDelay < 0 {
		return Config{}, fmt.Errorf("ROUTER_RETRY_DELAY_MS 不能是负数")
	}

	return cfg, nil
}

func (c Config) DebugLogging() bool {
	return c.LogLevel == "debug"
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
		return fmt.Errorf("%s 无效: %w", key, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s 必须使用 http 或 https", key)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s 必须包含主机名", key)
	}
	return nil
}

func getEnvInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数，当前值=%q", key, value)
	}
	return parsed, nil
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
