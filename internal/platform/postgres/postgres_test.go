package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/pay-pact/internal/platform/postgres"
)

func TestOpenValidatesOptionsBeforeConnecting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options postgres.Options
		wantErr error
	}{
		{name: "missing URL", options: postgres.Options{MaxConns: 1, PingTimeout: time.Second}, wantErr: postgres.ErrDatabaseURLRequired},
		{name: "missing max connections", options: postgres.Options{URL: "postgres://localhost/paypact", PingTimeout: time.Second}, wantErr: postgres.ErrInvalidPoolConfig},
		{name: "negative minimum", options: postgres.Options{URL: "postgres://localhost/paypact", MaxConns: 1, MinConns: -1, PingTimeout: time.Second}, wantErr: postgres.ErrInvalidPoolConfig},
		{name: "minimum exceeds maximum", options: postgres.Options{URL: "postgres://localhost/paypact", MaxConns: 1, MinConns: 2, PingTimeout: time.Second}, wantErr: postgres.ErrInvalidPoolConfig},
		{name: "missing timeout", options: postgres.Options{URL: "postgres://localhost/paypact", MaxConns: 1}, wantErr: postgres.ErrInvalidPoolConfig},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pool, err := postgres.Open(context.Background(), test.options)
			if pool != nil {
				pool.Close()
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Open() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
