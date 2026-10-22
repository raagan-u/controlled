package config

import (
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestLoadUsesValidatedDefaults(t *testing.T) {
	t.Parallel()

	configuration, err := load(mapLookup(map[string]string{
		"DATABASE_URL": "postgres://paypact:secret@localhost/paypact",
	}))
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if configuration.HTTPAddress != ":8080" ||
		configuration.LogLevel != slog.LevelInfo ||
		configuration.ShutdownTimeout != 10*time.Second ||
		configuration.DatabaseMaxConns != 10 ||
		configuration.DatabaseMinConns != 1 ||
		configuration.DatabasePingTimeout != 5*time.Second {
		t.Fatalf("load() = %#v", configuration)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Parallel()

	configuration, err := load(mapLookup(map[string]string{
		"DATABASE_URL":             "postgres://localhost/paypact",
		"PAYPACT_HTTP_ADDR":        "127.0.0.1:9090",
		"PAYPACT_LOG_LEVEL":        "debug",
		"PAYPACT_SHUTDOWN_TIMEOUT": "3s",
		"PAYPACT_DB_PING_TIMEOUT":  "2s",
		"PAYPACT_DB_MAX_CONNS":     "20",
		"PAYPACT_DB_MIN_CONNS":     "4",
	}))
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if configuration.HTTPAddress != "127.0.0.1:9090" ||
		configuration.LogLevel != slog.LevelDebug ||
		configuration.ShutdownTimeout != 3*time.Second ||
		configuration.DatabasePingTimeout != 2*time.Second ||
		configuration.DatabaseMaxConns != 20 ||
		configuration.DatabaseMinConns != 4 {
		t.Fatalf("load() = %#v", configuration)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  map[string]string
		wantErr error
	}{
		{name: "missing database", values: map[string]string{}, wantErr: ErrDatabaseURLRequired},
		{name: "log level", values: map[string]string{"DATABASE_URL": "postgres://db", "PAYPACT_LOG_LEVEL": "TRACE"}, wantErr: ErrInvalidConfig},
		{name: "duration", values: map[string]string{"DATABASE_URL": "postgres://db", "PAYPACT_DB_PING_TIMEOUT": "0s"}, wantErr: ErrInvalidConfig},
		{name: "empty address", values: map[string]string{"DATABASE_URL": "postgres://db", "PAYPACT_HTTP_ADDR": " "}, wantErr: ErrInvalidConfig},
		{name: "connection count", values: map[string]string{"DATABASE_URL": "postgres://db", "PAYPACT_DB_MAX_CONNS": "0"}, wantErr: ErrInvalidConfig},
		{name: "minimum exceeds maximum", values: map[string]string{"DATABASE_URL": "postgres://db", "PAYPACT_DB_MAX_CONNS": "2", "PAYPACT_DB_MIN_CONNS": "3"}, wantErr: ErrInvalidConfig},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := load(mapLookup(test.values))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("load() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func mapLookup(values map[string]string) lookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
