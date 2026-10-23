package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/platform"
	"github.com/raagan-u/pay-pact/internal/platform/config"
	"github.com/raagan-u/pay-pact/internal/platform/httpapi"
	platformpostgres "github.com/raagan-u/pay-pact/internal/platform/postgres"
	"github.com/raagan-u/pay-pact/internal/wallet"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("PayPact API stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	configuration, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: configuration.LogLevel})))

	database, err := platformpostgres.Open(ctx, platformpostgres.Options{
		URL:         configuration.DatabaseURL,
		MaxConns:    configuration.DatabaseMaxConns,
		MinConns:    configuration.DatabaseMinConns,
		PingTimeout: configuration.DatabasePingTimeout,
	})
	if err != nil {
		return err
	}
	defer database.Close()
	if err := platformpostgres.Migrate(ctx, database); err != nil {
		return err
	}

	store := wallet.NewPostgresProvisioningStore(database)
	onboarding := wallet.NewOnboardingService(
		platform.NewUUIDGenerator(),
		platform.SystemClock{},
		store,
	)
	sessions := identity.NewSessionService(
		platform.NewUUIDGenerator(),
		platform.NewTokenGenerator(),
		platform.SystemClock{},
		identity.NewPostgresSessionStore(database),
		24*time.Hour,
	)
	sessionHandler := identity.NewSessionHandler(sessions)

	mux := http.NewServeMux()
	mux.Handle("POST /v1/demo/users", wallet.NewOnboardingHandler(onboarding))
	mux.HandleFunc("POST /v1/demo/sessions", sessionHandler.Start)
	mux.HandleFunc("DELETE /v1/sessions/current", sessionHandler.End)
	health := httpapi.NewHealthHandler(database, configuration.DatabasePingTimeout)
	mux.HandleFunc("GET /healthz", health.Liveness)
	mux.HandleFunc("GET /readyz", health.Readiness)

	server := &http.Server{
		Addr:              configuration.HTTPAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("PayPact API listening", "address", configuration.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), configuration.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return err
		}
	}
	return nil
}
