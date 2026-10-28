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

	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/platform"
	"github.com/raagan-u/controlled/internal/platform/config"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
	platformpostgres "github.com/raagan-u/controlled/internal/platform/postgres"
	"github.com/raagan-u/controlled/internal/wallet"
	"github.com/raagan-u/controlled/internal/walletops"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("Controlled API stopped", "error", err)
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
	identityStore := identity.NewPostgresSessionStore(database)
	sessions := identity.NewSessionService(
		platform.NewUUIDGenerator(),
		platform.NewTokenGenerator(),
		platform.SystemClock{},
		identityStore,
		24*time.Hour,
	)
	sessionHandler := identity.NewSessionHandler(sessions)
	googleAuthentication := identity.NewGoogleAuthenticationService(
		identity.NewGoogleVerifier(configuration.GoogleClientID),
		identityStore,
		googleUserProvisioner{onboarding: onboarding},
		sessions,
	)
	googleAuthenticationHandler := identity.NewGoogleAuthenticationHandler(googleAuthentication)
	walletReadHandler := wallet.NewReadHandler(store)
	fakeCustody := custody.NewFake(platform.NewUUIDGenerator())
	walletOperations := walletops.NewService(
		platform.NewUUIDGenerator(),
		platform.SystemClock{},
		walletops.NewPostgresRepository(database, ledger.NewPostgresBook(database)),
		fakeCustody,
		walletops.AllowAllPolicy{},
	)
	walletOperationHandler := walletops.NewHandler(walletOperations)

	mux := http.NewServeMux()
	mux.Handle("POST /v1/demo/users", wallet.NewOnboardingHandler(onboarding))
	mux.HandleFunc("POST /v1/demo/sessions", sessionHandler.Start)
	mux.Handle("POST /v1/auth/google", googleAuthenticationHandler)
	mux.HandleFunc("DELETE /v1/sessions/current", sessionHandler.End)
	mux.Handle("GET /v1/users/current", identity.RequireSession(sessions, http.HandlerFunc(identity.CurrentUserHandler)))
	mux.Handle("GET /v1/wallet", identity.RequireSession(sessions, http.HandlerFunc(walletReadHandler.Wallet)))
	mux.Handle("GET /v1/wallet/transactions", identity.RequireSession(sessions, http.HandlerFunc(walletReadHandler.Transactions)))
	mux.Handle("POST /v1/wallet/funding-intents", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.CreateFunding)))
	mux.Handle("POST /v1/demo/funding-intents/{id}/confirm", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.ConfirmFunding)))
	mux.Handle("POST /v1/wallet/internal-transfers", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.CreateTransfer)))
	mux.Handle("POST /v1/wallet/withdrawals", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.CreateWithdrawal)))
	mux.Handle("POST /v1/demo/withdrawals/{id}/settle", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.SettleWithdrawal)))
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
		slog.Info("Controlled API listening", "address", configuration.HTTPAddress)
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
