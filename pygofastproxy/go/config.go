package main

import (
	"log"
	"os"
	"strconv"
	"time"
)
// Config holds the configuration settings for the proxy server.
type Config struct {
	MaxConnsPerHost      int
	MaxConnWaitTimeout   time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	MaxIdleConnDuration  time.Duration
	ReadBufferSize       int
	WriteBufferSize      int
	RateLimitRPS         int
	MaxRequestBodySize   int
	TLSCertFile          string
	TLSKeyFile           string
	CORSAllowCredentials bool
}
// LoadConfig reads configuration from environment variables and returns a Config struct with the settings.
func LoadConfig() *Config {
	config := &Config{
		MaxConnsPerHost:    1000,
		MaxConnWaitTimeout: 5 * time.Second,
		ReadTimeout:        10 * time.Second,
		WriteTimeout:       10 * time.Second,
		MaxIdleConnDuration: 60 * time.Second,
		ReadBufferSize:     16 * 1024,
		WriteBufferSize:    16 * 1024,
		RateLimitRPS:       1000,
		MaxRequestBodySize: 10 * 1024 * 1024,
	}

	if val := os.Getenv("PROXY_MAX_CONNS_PER_HOST"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			config.MaxConnsPerHost = parsed
		} else if err == nil {
			log.Printf("WARNING: PROXY_MAX_CONNS_PER_HOST must be > 0, using default %d", config.MaxConnsPerHost)
		}
	}
	if val := os.Getenv("PROXY_MAX_CONN_WAIT_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			config.MaxConnWaitTimeout = parsed
		}
	}
	if val := os.Getenv("PROXY_READ_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			config.ReadTimeout = parsed
		} else if err == nil {
			log.Printf("WARNING: PROXY_READ_TIMEOUT must be > 0, using default %s", config.ReadTimeout)
		}
	}
	if val := os.Getenv("PROXY_WRITE_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			config.WriteTimeout = parsed
		} else if err == nil {
			log.Printf("WARNING: PROXY_WRITE_TIMEOUT must be > 0, using default %s", config.WriteTimeout)
		}
	}
	if val := os.Getenv("PROXY_MAX_IDLE_CONN_DURATION"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			config.MaxIdleConnDuration = parsed
		}
	}
	if val := os.Getenv("PROXY_READ_BUFFER_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			config.ReadBufferSize = parsed
		}
	}
	if val := os.Getenv("PROXY_WRITE_BUFFER_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			config.WriteBufferSize = parsed
		}
	}
	if val := os.Getenv("PROXY_RATE_LIMIT_RPS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 {
			config.RateLimitRPS = parsed
		}
	}
	if val := os.Getenv("PROXY_MAX_REQUEST_BODY_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			config.MaxRequestBodySize = parsed
		}
	}

	config.TLSCertFile = os.Getenv("PROXY_TLS_CERT_FILE")
	config.TLSKeyFile = os.Getenv("PROXY_TLS_KEY_FILE")

	if val := os.Getenv("PROXY_CORS_ALLOW_CREDENTIALS"); val != "" {
		config.CORSAllowCredentials = val == "true" || val == "1"
	}

	return config
}
