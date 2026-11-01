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
		"DATABASE_URL": "postgres://controlled:secret@localhost/controlled",
	}))
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if configuration.HTTPAddress != ":8080" ||
		configuration.LogLevel != slog.LevelInfo ||
		configuration.ShutdownTimeout != 10*time.Second ||
		configuration.DatabaseMaxConns != 10 ||
		configuration.DatabaseMinConns != 1 ||
		configuration.DatabasePingTimeout != 5*time.Second ||
		configuration.GoogleClientID != defaultGoogleClientID ||
		configuration.CustodyProvider != CustodyProviderFake ||
		configuration.PayPalAPIBase != "https://api-m.sandbox.paypal.com" ||
		configuration.AIProvider != AIProviderDisabled ||
		configuration.AIBaseURL != "https://openrouter.ai/api/v1" ||
		configuration.AITimeout != 8*time.Second ||
		!configuration.AIRequireParameters ||
		configuration.ExpoPushURL != "https://exp.host/--/api/v2/push/send" ||
		configuration.ExpoPushTimeout != 5*time.Second {
		t.Fatalf("load() = %#v", configuration)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Parallel()

	configuration, err := load(mapLookup(map[string]string{
		"DATABASE_URL":                     "postgres://localhost/controlled",
		"CONTROLLED_HTTP_ADDR":             "127.0.0.1:9090",
		"CONTROLLED_LOG_LEVEL":             "debug",
		"CONTROLLED_SHUTDOWN_TIMEOUT":      "3s",
		"CONTROLLED_DB_PING_TIMEOUT":       "2s",
		"CONTROLLED_DB_MAX_CONNS":          "20",
		"CONTROLLED_DB_MIN_CONNS":          "4",
		"CONTROLLED_GOOGLE_CLIENT_ID":      "google-client-id",
		"CONTROLLED_CUSTODY_PROVIDER":      "paypal_sandbox",
		"PAYPAL_API_BASE":                  "https://sandbox.example.test/",
		"PAYPAL_CLIENT_ID":                 "paypal-client-id",
		"PAYPAL_CLIENT_SECRET":             "paypal-secret",
		"PAYPAL_RETURN_URL":                "controlled://paypal/return",
		"PAYPAL_CANCEL_URL":                "controlled://paypal/cancel",
		"CONTROLLED_AI_PROVIDER":           "openai_compatible",
		"CONTROLLED_AI_BASE_URL":           "https://models.example.test/v1/",
		"CONTROLLED_AI_MODEL":              "example/model",
		"CONTROLLED_AI_API_KEY":            "ai-secret",
		"CONTROLLED_AI_TIMEOUT":            "8s",
		"CONTROLLED_AI_REQUIRE_PARAMETERS": "false",
		"CONTROLLED_EXPO_PUSH_URL":         "https://push.example.test/send",
		"CONTROLLED_EXPO_ACCESS_TOKEN":     "push-secret",
		"CONTROLLED_EXPO_PUSH_TIMEOUT":     "4s",
	}))
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if configuration.HTTPAddress != "127.0.0.1:9090" ||
		configuration.LogLevel != slog.LevelDebug ||
		configuration.ShutdownTimeout != 3*time.Second ||
		configuration.DatabasePingTimeout != 2*time.Second ||
		configuration.DatabaseMaxConns != 20 ||
		configuration.DatabaseMinConns != 4 ||
		configuration.GoogleClientID != "google-client-id" ||
		configuration.CustodyProvider != CustodyProviderPayPalSandbox ||
		configuration.PayPalAPIBase != "https://sandbox.example.test" ||
		configuration.PayPalClientID != "paypal-client-id" {
		t.Fatalf("load() = %#v", configuration)
	}
	if configuration.AIProvider != AIProviderOpenAICompatible ||
		configuration.AIBaseURL != "https://models.example.test/v1" ||
		configuration.AIModel != "example/model" || configuration.AIAPIKey != "ai-secret" ||
		configuration.AITimeout != 8*time.Second || configuration.AIRequireParameters {
		t.Fatalf("load() AI = %#v", configuration)
	}
	if configuration.ExpoPushURL != "https://push.example.test/send" ||
		configuration.ExpoAccessToken != "push-secret" || configuration.ExpoPushTimeout != 4*time.Second {
		t.Fatalf("load() push = %#v", configuration)
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
		{name: "log level", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_LOG_LEVEL": "TRACE"}, wantErr: ErrInvalidConfig},
		{name: "duration", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_DB_PING_TIMEOUT": "0s"}, wantErr: ErrInvalidConfig},
		{name: "empty address", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_HTTP_ADDR": " "}, wantErr: ErrInvalidConfig},
		{name: "empty Google client ID", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_GOOGLE_CLIENT_ID": " "}, wantErr: ErrInvalidConfig},
		{name: "connection count", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_DB_MAX_CONNS": "0"}, wantErr: ErrInvalidConfig},
		{name: "minimum exceeds maximum", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_DB_MAX_CONNS": "2", "CONTROLLED_DB_MIN_CONNS": "3"}, wantErr: ErrInvalidConfig},
		{name: "custody provider", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_CUSTODY_PROVIDER": "unknown"}, wantErr: ErrInvalidConfig},
		{name: "missing PayPal credentials", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_CUSTODY_PROVIDER": "paypal_sandbox"}, wantErr: ErrInvalidConfig},
		{name: "AI provider", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_AI_PROVIDER": "unknown"}, wantErr: ErrInvalidConfig},
		{name: "missing AI configuration", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_AI_PROVIDER": "openai_compatible"}, wantErr: ErrInvalidConfig},
		{name: "AI boolean", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_AI_REQUIRE_PARAMETERS": "sometimes"}, wantErr: ErrInvalidConfig},
		{name: "AI timeout exceeds response window", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_AI_PROVIDER": "openai_compatible", "CONTROLLED_AI_MODEL": "model", "CONTROLLED_AI_API_KEY": "secret", "CONTROLLED_AI_TIMEOUT": "10s"}, wantErr: ErrInvalidConfig},
		{name: "empty Expo push URL", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_EXPO_PUSH_URL": " "}, wantErr: ErrInvalidConfig},
		{name: "invalid Expo push timeout", values: map[string]string{"DATABASE_URL": "postgres://db", "CONTROLLED_EXPO_PUSH_TIMEOUT": "0s"}, wantErr: ErrInvalidConfig},
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
