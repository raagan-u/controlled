package main

import (
	"context"
	"errors"
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
	platformlogging "github.com/raagan-u/controlled/internal/platform/logging"
	platformpostgres "github.com/raagan-u/controlled/internal/platform/postgres"
	"github.com/raagan-u/controlled/internal/shopping"
	"github.com/raagan-u/controlled/internal/wallet"
	"github.com/raagan-u/controlled/internal/walletops"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger, _ := zap.NewProduction()
	zap.ReplaceGlobals(bootstrapLogger)
	defer func() { _ = bootstrapLogger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		zap.L().Error("Controlled API stopped", zap.Error(err))
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	configuration, err := config.Load()
	if err != nil {
		return err
	}
	logger, err := platformlogging.New(configuration.LogLevel)
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(logger)
	defer func() { _ = logger.Sync() }()
	logger.Info("Controlled API starting",
		zap.String("address", configuration.HTTPAddress),
		zap.String("log_level", configuration.LogLevel.String()),
		zap.String("custody_provider", configuration.CustodyProvider),
		zap.String("ai_provider", configuration.AIProvider),
		zap.Bool("shopping_enabled", configuration.Channel3APIKey != ""),
	)

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
	logger.Info("Database connection established",
		zap.Int32("max_connections", configuration.DatabaseMaxConns),
		zap.Int32("min_connections", configuration.DatabaseMinConns),
	)
	logFeatureStatus(logger, "database", "postgres", true)
	if err := platformpostgres.Migrate(ctx, database); err != nil {
		return err
	}
	logger.Info("Database migrations completed")

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
	logFeatureStatus(logger, "authentication.google", "google", true)
	logFeatureStatus(logger, "authentication.password", "bcrypt", true)
	logFeatureStatus(logger, "authentication.demo_sessions", "internal", true)
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
	logFeatureStatus(logger, "push_notifications", "expo", true,
		zap.Bool("authenticated", configuration.ExpoAccessToken != ""),
		zap.Duration("timeout", configuration.ExpoPushTimeout),
	)
	go pushService.Run(ctx)
	walletReadHandler := wallet.NewReadHandler(store)
	custodyProvider, err := configuredCustodyProvider(configuration)
	if err != nil {
		return err
	}
	logFeatureStatus(logger, "custody", configuration.CustodyProvider, true)
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
	logFeatureStatus(logger, "accountability", "internal", true)
	logFeatureStatus(logger, "pacts", "internal", true)
	aiModel, err := configuredAIModel(configuration)
	if err != nil {
		return err
	}
	logFeatureStatus(logger, "ai", configuration.AIProvider, aiModel != nil,
		zap.String("model", configuration.AIModel),
		zap.Duration("timeout", configuration.AITimeout),
	)
	aiService := controlledai.NewService(platform.SystemClock{}, store, pactService, accountabilityService, identityStore, aiModel).
		EnableAssessmentPersistence(platform.NewUUIDGenerator(), controlledai.NewPostgresAssessmentStore(database))
	aiHandler := controlledai.NewHandler(aiService)
	var productCatalog shopping.Catalog
	if configuration.Channel3APIKey != "" {
		productCatalog, err = shopping.NewChannel3Catalog(shopping.Channel3Options{
			APIBase: configuration.Channel3APIBase, APIKey: configuration.Channel3APIKey, Timeout: configuration.Channel3Timeout,
		})
		if err != nil {
			return err
		}
	}
	logFeatureStatus(logger, "shopping.price_checks", "channel3", productCatalog != nil,
		zap.Duration("timeout", configuration.Channel3Timeout),
	)
	shoppingHandler := shopping.NewHandler(shopping.NewService(productCatalog))
	paymentRequestService := paymentrequest.NewService(platform.NewUUIDGenerator(), platform.SystemClock{}, paymentrequest.NewPostgresRepository(database), walletOperations)
	paymentRequestHandler := paymentrequest.NewHandler(paymentRequestService).EnableNotifications(pushService)
	logFeatureStatus(logger, "payment_requests", "internal", true)
	logger.Info("Backend features initialized")
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
	mux.Handle("POST /v1/ai/transaction-assessments", identity.RequireSession(sessions, http.HandlerFunc(aiHandler.AssessTransaction)))
	mux.Handle("GET /v1/ai/pacts/{id}/insight", identity.RequireSession(sessions, http.HandlerFunc(aiHandler.PactInsight)))
	mux.Handle("GET /v1/ai/accountability/{id}/brief", identity.RequireSession(sessions, http.HandlerFunc(aiHandler.AccountabilityBrief)))
	mux.Handle("POST /v1/shopping/price-checks", identity.RequireSession(sessions, http.HandlerFunc(shoppingHandler.CheckPrice)))
	health := httpapi.NewHealthHandler(database, configuration.DatabasePingTimeout)
	mux.HandleFunc("GET /healthz", health.Liveness)
	mux.HandleFunc("GET /readyz", health.Readiness)

	server := &http.Server{
		Addr:              configuration.HTTPAddress,
		Handler:           httpapi.LogRequests(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("Controlled API listening", zap.String("address", configuration.HTTPAddress))
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("Controlled API shutdown started")
		shutdownContext, cancel := context.WithTimeout(context.Background(), configuration.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return err
		}
		logger.Info("Controlled API shutdown completed")
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

func logFeatureStatus(logger *zap.Logger, feature, provider string, enabled bool, fields ...zap.Field) {
	fields = append([]zap.Field{
		zap.String("feature", feature),
		zap.String("provider", provider),
		zap.Bool("enabled", enabled),
		zap.Bool("loaded", enabled),
	}, fields...)
	logger.Info("Backend feature status", fields...)
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
			zap.L().Error("Accountability maintenance failed", zap.Error(err))
		} else if result.TimedOut > 0 || result.Expired > 0 || result.Recovered > 0 {
			zap.L().Info("Accountability maintenance completed", zap.Int("timed_out", result.TimedOut), zap.Int("expired", result.Expired), zap.Int("recovered", result.Recovered))
		}
		if result, err := pactService.RunMaintenance(ctx); err != nil && !errors.Is(err, context.Canceled) {
			zap.L().Error("Pact maintenance failed", zap.Error(err))
		} else if result.Started > 0 || result.Completed > 0 {
			zap.L().Info("Pact maintenance completed", zap.Int("started", result.Started), zap.Int("completed", result.Completed))
		}
		if err := paymentRequests.Maintain(ctx); err != nil && !errors.Is(err, context.Canceled) {
			zap.L().Error("Payment request maintenance failed", zap.Error(err))
		}
		if result, err := walletOperations.ReconcilePendingWithdrawals(ctx, 100); err != nil && !errors.Is(err, context.Canceled) {
			zap.L().Error("Withdrawal reconciliation failed", zap.Error(err))
		} else if result.Completed > 0 || result.Failed > 0 {
			zap.L().Info("Withdrawal reconciliation completed", zap.Int("completed", result.Completed), zap.Int("failed", result.Failed), zap.Int("pending", result.Pending))
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
