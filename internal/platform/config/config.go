// Package config loads and validates Controlled process configuration.
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

const defaultGoogleClientID = "376706277927-s4mest4okp2a4par7m6tl69b9ut3rl7c.apps.googleusercontent.com"

type Config struct {
	HTTPAddress         string
	DatabaseURL         string
	LogLevel            slog.Level
	ShutdownTimeout     time.Duration
	DatabaseMaxConns    int32
	DatabaseMinConns    int32
	DatabasePingTimeout time.Duration
	GoogleClientID      string
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

	logLevel, err := parseLogLevel(value(lookup, "CONTROLLED_LOG_LEVEL", "INFO"))
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := parsePositiveDuration(lookup, "CONTROLLED_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	pingTimeout, err := parsePositiveDuration(lookup, "CONTROLLED_DB_PING_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	maxConns, err := parsePositiveInt32(lookup, "CONTROLLED_DB_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	minConns, err := parseNonNegativeInt32(lookup, "CONTROLLED_DB_MIN_CONNS", 1)
	if err != nil {
		return Config{}, err
	}
	if minConns > maxConns {
		return Config{}, fmt.Errorf("%w: CONTROLLED_DB_MIN_CONNS must not exceed CONTROLLED_DB_MAX_CONNS", ErrInvalidConfig)
	}
	httpAddress := strings.TrimSpace(value(lookup, "CONTROLLED_HTTP_ADDR", ":8080"))
	if httpAddress == "" {
		return Config{}, fmt.Errorf("%w: CONTROLLED_HTTP_ADDR", ErrInvalidConfig)
	}
	googleClientID := strings.TrimSpace(value(lookup, "CONTROLLED_GOOGLE_CLIENT_ID", defaultGoogleClientID))
	if googleClientID == "" {
		return Config{}, fmt.Errorf("%w: CONTROLLED_GOOGLE_CLIENT_ID", ErrInvalidConfig)
	}

	return Config{
		HTTPAddress:         httpAddress,
		DatabaseURL:         databaseURL,
		LogLevel:            logLevel,
		ShutdownTimeout:     shutdownTimeout,
		DatabaseMaxConns:    maxConns,
		DatabaseMinConns:    minConns,
		DatabasePingTimeout: pingTimeout,
		GoogleClientID:      googleClientID,
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
		return 0, fmt.Errorf("%w: CONTROLLED_LOG_LEVEL", ErrInvalidConfig)
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
