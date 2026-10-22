// Package config loads and validates PayPact process configuration.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrDatabaseURLRequired = errors.New("DATABASE_URL is required")
	ErrInvalidConfig       = errors.New("configuration value is invalid")
)

type Config struct {
	HTTPAddress         string
	DatabaseURL         string
	LogLevel            slog.Level
	ShutdownTimeout     time.Duration
	DatabaseMaxConns    int32
	DatabaseMinConns    int32
	DatabasePingTimeout time.Duration
}

func Load() (Config, error) {
	return load(os.LookupEnv)
}

type lookupEnv func(string) (string, bool)

func load(lookup lookupEnv) (Config, error) {
	databaseURL := strings.TrimSpace(value(lookup, "DATABASE_URL", ""))
	if databaseURL == "" {
		return Config{}, ErrDatabaseURLRequired
	}

	logLevel, err := parseLogLevel(value(lookup, "PAYPACT_LOG_LEVEL", "INFO"))
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := parsePositiveDuration(lookup, "PAYPACT_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	pingTimeout, err := parsePositiveDuration(lookup, "PAYPACT_DB_PING_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	maxConns, err := parsePositiveInt32(lookup, "PAYPACT_DB_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	minConns, err := parseNonNegativeInt32(lookup, "PAYPACT_DB_MIN_CONNS", 1)
	if err != nil {
		return Config{}, err
	}
	if minConns > maxConns {
		return Config{}, fmt.Errorf("%w: PAYPACT_DB_MIN_CONNS must not exceed PAYPACT_DB_MAX_CONNS", ErrInvalidConfig)
	}
	httpAddress := strings.TrimSpace(value(lookup, "PAYPACT_HTTP_ADDR", ":8080"))
	if httpAddress == "" {
		return Config{}, fmt.Errorf("%w: PAYPACT_HTTP_ADDR", ErrInvalidConfig)
	}

	return Config{
		HTTPAddress:         httpAddress,
		DatabaseURL:         databaseURL,
		LogLevel:            logLevel,
		ShutdownTimeout:     shutdownTimeout,
		DatabaseMaxConns:    maxConns,
		DatabaseMinConns:    minConns,
		DatabasePingTimeout: pingTimeout,
	}, nil
}

func value(lookup lookupEnv, key, fallback string) string {
	if found, ok := lookup(key); ok {
		return found
	}
	return fallback
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: PAYPACT_LOG_LEVEL", ErrInvalidConfig)
	}
}

func parsePositiveDuration(lookup lookupEnv, key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(value(lookup, key, fallback.String()))
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: %s", ErrInvalidConfig, key)
	}
	return parsed, nil
}

func parsePositiveInt32(lookup lookupEnv, key string, fallback int32) (int32, error) {
	parsed, err := parseInt32(lookup, key, fallback)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: %s", ErrInvalidConfig, key)
	}
	return parsed, nil
}

func parseNonNegativeInt32(lookup lookupEnv, key string, fallback int32) (int32, error) {
	parsed, err := parseInt32(lookup, key, fallback)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%w: %s", ErrInvalidConfig, key)
	}
	return parsed, nil
}

func parseInt32(lookup lookupEnv, key string, fallback int32) (int32, error) {
	raw := strings.TrimSpace(value(lookup, key, strconv.FormatInt(int64(fallback), 10)))
	parsed, err := strconv.ParseInt(raw, 10, 32)
	return int32(parsed), err
}
