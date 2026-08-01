package config

import (
	"fmt"
	"strconv"
	"time"
)

const defaultMaxResponseBytes int64 = 10 << 20

type Shared struct {
	TLSVerify        bool
	LogLevel         string
	ConnectTimeout   time.Duration
	RequestTimeout   time.Duration
	MaxResponseBytes int64
}

func LoadShared(getenv func(string) string) (Shared, []string, error) {
	cfg := Shared{
		LogLevel:         valueOr(getenv("ATLASSIAN_LOG_LEVEL"), "info"),
		ConnectTimeout:   5 * time.Second,
		RequestTimeout:   60 * time.Second,
		MaxResponseBytes: defaultMaxResponseBytes,
	}
	if raw := getenv("ATLASSIAN_TLS_VERIFY"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil || (raw != "true" && raw != "false") {
			return Shared{}, nil, fmt.Errorf("ATLASSIAN_TLS_VERIFY must be true or false")
		}
		cfg.TLSVerify = v
	}
	if raw := getenv("ATLASSIAN_CONNECT_TIMEOUT"); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 {
			return Shared{}, nil, fmt.Errorf("ATLASSIAN_CONNECT_TIMEOUT must be a positive duration")
		}
		cfg.ConnectTimeout = v
	}
	if raw := getenv("ATLASSIAN_REQUEST_TIMEOUT"); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 {
			return Shared{}, nil, fmt.Errorf("ATLASSIAN_REQUEST_TIMEOUT must be a positive duration")
		}
		cfg.RequestTimeout = v
	}
	if raw := getenv("ATLASSIAN_MAX_RESPONSE_BYTES"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			return Shared{}, nil, fmt.Errorf("ATLASSIAN_MAX_RESPONSE_BYTES must be a positive integer")
		}
		cfg.MaxResponseBytes = v
	}
	return cfg, nil, nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
