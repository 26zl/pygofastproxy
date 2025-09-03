package main

import (
	"os"
	"strconv"
	"time"
)

// Holds proxy configuration values
type Config struct {
	MaxConnsPerHost     int
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	MaxIdleConnDuration time.Duration
	ReadBufferSize      int
	WriteBufferSize     int
	RateLimitRPS        int
	EnableMetrics       bool
}

// Loads configuration from environment variables with defaults
func LoadConfig() *Config {
	config := &Config{
		MaxConnsPerHost:     1000,
		ReadTimeout:         10 * time.Second,
		WriteTimeout:        10 * time.Second,
		MaxIdleConnDuration: 60 * time.Second,
		ReadBufferSize:      16 * 1024, // 16KB
		WriteBufferSize:     16 * 1024, // 16KB
		RateLimitRPS:        1000,
		EnableMetrics:       true,
	}

	// Override with environment variables
	if val := os.Getenv("PROXY_MAX_CONNS_PER_HOST"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxConnsPerHost = parsed
		}
	}
	if val := os.Getenv("PROXY_READ_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.ReadTimeout = parsed
		}
	}
	if val := os.Getenv("PROXY_WRITE_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.WriteTimeout = parsed
		}
	}
	if val := os.Getenv("PROXY_MAX_IDLE_CONN_DURATION"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			config.MaxIdleConnDuration = parsed
		}
	}
	if val := os.Getenv("PROXY_RATE_LIMIT_RPS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.RateLimitRPS = parsed
		}
	}
	if val := os.Getenv("PROXY_ENABLE_METRICS"); val != "" {
		config.EnableMetrics = val == "true"
	}

	return config
}
