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

	"github.com/raagan-u/controlled/internal/accountability"
	controlledai "github.com/raagan-u/controlled/internal/ai"
	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/notification"
	"github.com/raagan-u/controlled/internal/pact"
	"github.com/raagan-u/controlled/internal/paymentrequest"
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
	passwordAuthentication := identity.NewPasswordAuthenticationService(
		identityStore,
		googleUserProvisioner{onboarding: onboarding},
		identity.BcryptPasswordHasher{},
		sessions,
	)
	passwordAuthenticationHandler := identity.NewPasswordAuthenticationHandler(passwordAuthentication)
	recipientHandler := identity.NewRecipientHandler(identityStore)
	profileHandler := identity.NewProfileHandler(identityStore)
	pushSender, err := notification.NewExpoSender(
		configuration.ExpoPushURL, configuration.ExpoAccessToken,
		&http.Client{Timeout: configuration.ExpoPushTimeout},
	)
	if err != nil {
		return err
	}
	pushService := notification.NewService(notification.NewPostgresRepository(database), pushSender)
	pushHandler := notification.NewHandler(pushService)
	go pushService.Run(ctx)
	walletReadHandler := wallet.NewReadHandler(store)
	custodyProvider, err := configuredCustodyProvider(configuration)
	if err != nil {
		return err
	}
	accountabilityService := accountability.NewService(
		platform.NewUUIDGenerator(),
		platform.SystemClock{},
		accountability.NewPostgresRepository(database),
	)
	walletOperations := walletops.NewService(
		platform.NewUUIDGenerator(),
		platform.SystemClock{},
		walletops.NewPostgresRepository(database, ledger.NewPostgresBook(database)),
		custodyProvider,
		walletops.AllowAllPolicy{},
	).EnableAccountability(accountabilityService).EnableNotifications(pushService).EnableUserDirectory(identityStore)
	accountabilityService.EnableExecutor(walletOperations)
	walletOperationHandler := walletops.NewHandler(walletOperations).EnableNotifications(pushService)
	accountabilityHandler := accountability.NewHandler(accountabilityService).EnableNotifications(pushService)
	pactService := pact.NewService(
		platform.NewUUIDGenerator(),
		platform.SystemClock{},
		pact.NewPostgresRepository(database),
	)
	pactHandler := pact.NewHandler(pactService)
	aiModel, err := configuredAIModel(configuration)
	if err != nil {
		return err
	}
	aiService := controlledai.NewService(platform.SystemClock{}, store, pactService, accountabilityService, identityStore, aiModel)
	aiHandler := controlledai.NewHandler(aiService)
	paymentRequestService := paymentrequest.NewService(platform.NewUUIDGenerator(), platform.SystemClock{}, paymentrequest.NewPostgresRepository(database), walletOperations)
	paymentRequestHandler := paymentrequest.NewHandler(paymentRequestService).EnableNotifications(pushService)
	go runMaintenance(ctx, accountabilityService, pactService, paymentRequestService, walletOperations)

	mux := http.NewServeMux()
	mux.Handle("POST /v1/demo/users", wallet.NewOnboardingHandler(onboarding))
	mux.HandleFunc("POST /v1/demo/sessions", sessionHandler.Start)
	mux.Handle("POST /v1/auth/google", googleAuthenticationHandler)
	mux.HandleFunc("POST /v1/auth/password/register", passwordAuthenticationHandler.Register)
	mux.HandleFunc("POST /v1/auth/password/login", passwordAuthenticationHandler.Login)
	mux.HandleFunc("DELETE /v1/sessions/current", sessionHandler.End)
	mux.Handle("GET /v1/users/current", identity.RequireSession(sessions, http.HandlerFunc(identity.CurrentUserHandler)))
	mux.Handle("PATCH /v1/users/current", identity.RequireSession(sessions, http.HandlerFunc(profileHandler.Update)))
	mux.Handle("GET /v1/users/recipients", identity.RequireSession(sessions, http.HandlerFunc(recipientHandler.Recipients)))
	mux.Handle("POST /v1/push-devices", identity.RequireSession(sessions, http.HandlerFunc(pushHandler.Register)))
	mux.Handle("DELETE /v1/push-devices", identity.RequireSession(sessions, http.HandlerFunc(pushHandler.Unregister)))
	mux.Handle("GET /v1/wallet", identity.RequireSession(sessions, http.HandlerFunc(walletReadHandler.Wallet)))
	mux.Handle("GET /v1/wallet/transactions", identity.RequireSession(sessions, http.HandlerFunc(walletReadHandler.Transactions)))
	mux.Handle("GET /v1/wallet/transactions/{id}", identity.RequireSession(sessions, http.HandlerFunc(walletReadHandler.Transaction)))
	mux.Handle("POST /v1/wallet/funding-intents", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.CreateFunding)))
	mux.Handle("POST /v1/wallet/funding-intents/{id}/capture", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.ConfirmFunding)))
	mux.Handle("POST /v1/demo/funding-intents/{id}/confirm", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.ConfirmFunding)))
	mux.Handle("POST /v1/wallet/internal-transfers", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.CreateTransfer)))
	mux.Handle("POST /v1/wallet/payment-requests", identity.RequireSession(sessions, http.HandlerFunc(paymentRequestHandler.Create)))
	mux.Handle("GET /v1/wallet/payment-requests", identity.RequireSession(sessions, http.HandlerFunc(paymentRequestHandler.List)))
	mux.Handle("GET /v1/wallet/payment-requests/{id}", identity.RequireSession(sessions, http.HandlerFunc(paymentRequestHandler.Detail)))
	mux.Handle("POST /v1/wallet/payment-requests/{id}/pay", identity.RequireSession(sessions, http.HandlerFunc(paymentRequestHandler.Pay)))
	mux.Handle("POST /v1/wallet/payment-requests/{id}/decline", identity.RequireSession(sessions, http.HandlerFunc(paymentRequestHandler.Decline)))
	mux.Handle("POST /v1/wallet/payment-requests/{id}/cancel", identity.RequireSession(sessions, http.HandlerFunc(paymentRequestHandler.Cancel)))
	mux.Handle("POST /v1/wallet/withdrawals", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.CreateWithdrawal)))
	mux.Handle("POST /v1/wallet/withdrawals/{id}/reconcile", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.ReconcileWithdrawal)))
	mux.Handle("POST /v1/demo/withdrawals/{id}/settle", identity.RequireSession(sessions, http.HandlerFunc(walletOperationHandler.SettleWithdrawal)))
	mux.Handle("POST /v1/accountability/invitations", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.Invite)))
	mux.Handle("GET /v1/accountability/invitations", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.Invitations)))
	mux.Handle("POST /v1/accountability/invitations/{id}/accept", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.Accept)))
	mux.Handle("POST /v1/accountability/invitations/{id}/decline", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.Decline)))
	mux.Handle("POST /v1/accountability/circles", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.CreateCircle)))
	mux.Handle("GET /v1/accountability/circles", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.Circles)))
	mux.Handle("GET /v1/accountability/requests", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.Requests)))
	mux.Handle("GET /v1/accountability/requests/{id}", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.Request)))
	mux.Handle("POST /v1/accountability/requests/{id}/approve", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.ApproveRequest)))
	mux.Handle("POST /v1/accountability/requests/{id}/deny", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.DenyRequest)))
	mux.Handle("POST /v1/accountability/requests/{id}/cancel", identity.RequireSession(sessions, http.HandlerFunc(accountabilityHandler.CancelRequest)))
	mux.Handle("POST /v1/pacts", identity.RequireSession(sessions, http.HandlerFunc(pactHandler.CreateDraft)))
	mux.Handle("GET /v1/pacts", identity.RequireSession(sessions, http.HandlerFunc(pactHandler.Pacts)))
	mux.Handle("GET /v1/pacts/{id}", identity.RequireSession(sessions, http.HandlerFunc(pactHandler.Pact)))
	mux.Handle("PUT /v1/pacts/{id}", identity.RequireSession(sessions, http.HandlerFunc(pactHandler.UpdateDraft)))
	mux.Handle("POST /v1/pacts/{id}/confirm", identity.RequireSession(sessions, http.HandlerFunc(pactHandler.Confirm)))
	mux.Handle("POST /v1/pacts/{id}/stake", identity.RequireSession(sessions, http.HandlerFunc(pactHandler.ReserveStake)))
	mux.Handle("POST /v1/pacts/{id}/cancel", identity.RequireSession(sessions, http.HandlerFunc(pactHandler.Cancel)))
	mux.Handle("POST /v1/ai/pact-suggestions", identity.RequireSession(sessions, http.HandlerFunc(aiHandler.SuggestPact)))
	mux.Handle("GET /v1/ai/pacts/{id}/insight", identity.RequireSession(sessions, http.HandlerFunc(aiHandler.PactInsight)))
	mux.Handle("GET /v1/ai/accountability/{id}/brief", identity.RequireSession(sessions, http.HandlerFunc(aiHandler.AccountabilityBrief)))
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

func configuredAIModel(configuration config.Config) (controlledai.Model, error) {
	if configuration.AIProvider == config.AIProviderDisabled {
		return nil, nil
	}
	return controlledai.NewOpenAICompatibleModel(controlledai.OpenAICompatibleOptions{
		BaseURL: configuration.AIBaseURL, Model: configuration.AIModel,
		APIKey: configuration.AIAPIKey, Timeout: configuration.AITimeout,
		RequireParameters: configuration.AIRequireParameters,
	})
}

func configuredCustodyProvider(configuration config.Config) (custody.Provider, error) {
	if configuration.CustodyProvider == config.CustodyProviderPayPalSandbox {
		return custody.NewPayPal(custody.PayPalOptions{
			APIBase: configuration.PayPalAPIBase, ClientID: configuration.PayPalClientID,
			ClientSecret: configuration.PayPalClientSecret, ReturnURL: configuration.PayPalReturnURL,
			CancelURL: configuration.PayPalCancelURL,
		})
	}
	return custody.NewFake(platform.NewUUIDGenerator()), nil
}

func runMaintenance(ctx context.Context, accountabilityService *accountability.Service, pactService *pact.Service, paymentRequests *paymentrequest.Service, walletOperations *walletops.Service) {
	run := func() {
		if result, err := accountabilityService.RunMaintenance(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("Accountability maintenance failed", "error", err)
		} else if result.TimedOut > 0 || result.Expired > 0 || result.Recovered > 0 {
			slog.Info("Accountability maintenance completed", "timedOut", result.TimedOut, "expired", result.Expired, "recovered", result.Recovered)
		}
		if result, err := pactService.RunMaintenance(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("Pact maintenance failed", "error", err)
		} else if result.Started > 0 || result.Completed > 0 {
			slog.Info("Pact maintenance completed", "started", result.Started, "completed", result.Completed)
		}
		if err := paymentRequests.Maintain(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("Payment request maintenance failed", "error", err)
		}
		if result, err := walletOperations.ReconcilePendingWithdrawals(ctx, 100); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("Withdrawal reconciliation failed", "error", err)
		} else if result.Completed > 0 || result.Failed > 0 {
			slog.Info("Withdrawal reconciliation completed", "completed", result.Completed, "failed", result.Failed, "pending", result.Pending)
		}
	}
	run()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
