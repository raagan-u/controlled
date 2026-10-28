// Package postgres owns PostgreSQL process setup for Controlled.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDatabaseURLRequired = errors.New("PostgreSQL URL is required")
	ErrInvalidPoolConfig   = errors.New("PostgreSQL pool configuration is invalid")
)

type Options struct {
	URL         string
	MaxConns    int32
	MinConns    int32
	PingTimeout time.Duration
}

func Open(ctx context.Context, options Options) (*pgxpool.Pool, error) {
	if strings.TrimSpace(options.URL) == "" {
		return nil, ErrDatabaseURLRequired
	}
	if options.MaxConns <= 0 || options.MinConns < 0 || options.MinConns > options.MaxConns || options.PingTimeout <= 0 {
		return nil, ErrInvalidPoolConfig
	}

	poolConfig, err := pgxpool.ParseConfig(options.URL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration: %w", err)
	}
	poolConfig.MaxConns = options.MaxConns
	poolConfig.MinConns = options.MinConns

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL pool: %w", err)
	}

	pingContext, cancel := context.WithTimeout(ctx, options.PingTimeout)
	defer cancel()
	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return pool, nil
}
