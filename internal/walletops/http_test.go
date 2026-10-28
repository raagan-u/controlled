package walletops_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/money"
	"github.com/raagan-u/controlled/internal/walletops"
)

func TestWalletOperationHandlers(t *testing.T) {
	t.Parallel()

	user, err := identity.NewUser("user-1", "founder@example.com", "Raagan", time.Now())
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}
	amount, err := money.New(2500, "USD")
	if err != nil {
		t.Fatalf("money.New() error = %v", err)
	}
	commands := stubCommands{amount: amount}
	handler := walletops.NewHandler(commands)
	mux := http.NewServeMux()
	mux.Handle("POST /v1/wallet/funding-intents", identity.RequireSession(operationSessionManager{user: user}, http.HandlerFunc(handler.CreateFunding)))
	mux.Handle("POST /v1/demo/funding-intents/{id}/confirm", identity.RequireSession(operationSessionManager{user: user}, http.HandlerFunc(handler.ConfirmFunding)))
	mux.Handle("POST /v1/wallet/internal-transfers", identity.RequireSession(operationSessionManager{user: user}, http.HandlerFunc(handler.CreateTransfer)))
	mux.Handle("POST /v1/wallet/withdrawals", identity.RequireSession(operationSessionManager{user: user}, http.HandlerFunc(handler.CreateWithdrawal)))
	mux.Handle("POST /v1/demo/withdrawals/{id}/settle", identity.RequireSession(operationSessionManager{user: user}, http.HandlerFunc(handler.SettleWithdrawal)))

	tests := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{name: "create funding", path: "/v1/wallet/funding-intents", body: `{"amountMinor":2500,"currency":"USD"}`, status: http.StatusCreated},
		{name: "confirm funding", path: "/v1/demo/funding-intents/funding-1/confirm", body: `{}`, status: http.StatusOK},
		{name: "internal transfer", path: "/v1/wallet/internal-transfers", body: `{"recipientEmail":"friend@example.com","amountMinor":2500,"currency":"USD"}`, status: http.StatusCreated},
		{name: "withdrawal", path: "/v1/wallet/withdrawals", body: `{"destination":"paypal@example.com","amountMinor":2500,"currency":"USD"}`, status: http.StatusAccepted},
		{name: "settle withdrawal", path: "/v1/demo/withdrawals/withdrawal-1/settle", body: `{"succeeded":true}`, status: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer test-token")
			request.Header.Set("Idempotency-Key", test.name)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}

type stubCommands struct {
	amount money.Money
}

func (s stubCommands) CreateFunding(context.Context, string, identity.UserID, int64, string) (walletops.FundingIntent, error) {
	return walletops.FundingIntent{
		ID: "funding-1", Amount: s.amount, Status: walletops.FundingAwaitingConfirmation,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil
}

func (s stubCommands) ConfirmFunding(context.Context, identity.UserID, walletops.OperationID) (walletops.FundingIntent, error) {
	return walletops.FundingIntent{
		ID: "funding-1", Amount: s.amount, Status: walletops.FundingCompleted,
		CreatedAt: time.Now(), UpdatedAt: time.Now(), CompletedAt: time.Now(),
	}, nil
}

func (s stubCommands) Transfer(context.Context, string, identity.UserID, string, int64, string) (walletops.Transfer, error) {
	return walletops.Transfer{
		ID: "transfer-1", RecipientID: "user-2", RecipientEmail: "friend@example.com",
		Amount: s.amount, CreatedAt: time.Now(),
	}, nil
}

func (s stubCommands) CreateWithdrawal(context.Context, string, identity.UserID, string, int64, string) (walletops.Withdrawal, error) {
	return walletops.Withdrawal{
		ID: "withdrawal-1", Destination: "paypal@example.com", Amount: s.amount,
		Status: walletops.WithdrawalPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil
}

func (s stubCommands) SimulateWithdrawalSettlement(context.Context, identity.UserID, walletops.OperationID, bool) (walletops.Withdrawal, error) {
	return walletops.Withdrawal{
		ID: "withdrawal-1", Destination: "paypal@example.com", Amount: s.amount,
		Status: walletops.WithdrawalCompleted, CreatedAt: time.Now(), UpdatedAt: time.Now(), CompletedAt: time.Now(),
	}, nil
}

type operationSessionManager struct{ user identity.User }

func (s operationSessionManager) Start(context.Context, string) (identity.StartedSession, error) {
	return identity.StartedSession{}, nil
}

func (s operationSessionManager) Authenticate(context.Context, string) (identity.User, error) {
	return s.user, nil
}

func (s operationSessionManager) End(context.Context, string) error { return nil }
