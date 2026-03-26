package webui

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	ListenAddr     string
	APIHost        string
	ClientID       string
	ClientSecret   string
	DefaultRoot    string
	DefaultLinkMode string
	RequestTimeout time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:     envOrDefault("HALAL_WEB_ADDR", ":8080"),
		APIHost:        envOrDefault("HALAL_API_HOST", "openapi.2dland.cn"),
		ClientID:       os.Getenv("HALAL_CLIENT_ID"),
		ClientSecret:   os.Getenv("HALAL_CLIENT_SECRET"),
		DefaultRoot:    envOrDefault("HALAL_DEFAULT_ROOT", "/"),
		DefaultLinkMode: normalizeLinkMode(envOrDefault("HALAL_WEB_LINK_MODE", "redirect")),
		RequestTimeout: 15 * time.Second,
	}
	if cfg.ClientID == "" {
		return Config{}, fmt.Errorf("HALAL_CLIENT_ID is required")
	}
	if cfg.ClientSecret == "" {
		return Config{}, fmt.Errorf("HALAL_CLIENT_SECRET is required")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func normalizeLinkMode(mode string) string {
	switch mode {
	case "proxy":
		return "proxy"
	default:
		return "redirect"
	}
}
