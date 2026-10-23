package wallet_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/money"
	"github.com/raagan-u/pay-pact/internal/wallet"
)

func TestWalletReadHandlerReturnsAuthenticatedWallet(t *testing.T) {
	t.Parallel()

	user, err := identity.NewUser("user-1", "founder@example.com", "Raagan", time.Now())
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}
	walletValue, err := wallet.NewWallet("wallet-1", user.ID(), "USD", wallet.AccountSet{
		Available:         "available-1",
		Pending:           "pending-1",
		CommitmentReserve: "reserve-1",
	}, time.Now())
	if err != nil {
		t.Fatalf("NewWallet() error = %v", err)
	}
	zero, err := money.New(0, "USD")
	if err != nil {
		t.Fatalf("money.New() error = %v", err)
	}
	snapshot, err := wallet.NewSnapshot(walletValue, wallet.BalanceSet{
		Available: zero, Pending: zero, CommitmentReserve: zero,
	})
	if err != nil {
		t.Fatalf("NewSnapshot() error = %v", err)
	}

	handler := wallet.NewReadHandler(stubWalletReader{snapshot: snapshot})
	protected := identity.RequireSession(walletSessionManager{user: user}, http.HandlerFunc(handler.Wallet))
	request := httptest.NewRequest(http.MethodGet, "/v1/wallet", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Wallet struct {
			ID       string `json:"id"`
			Balances struct {
				Available struct {
					AmountMinor int64  `json:"amountMinor"`
					Currency    string `json:"currency"`
				} `json:"available"`
			} `json:"balances"`
		} `json:"wallet"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Wallet.ID != "wallet-1" ||
		payload.Wallet.Balances.Available.AmountMinor != 0 ||
		payload.Wallet.Balances.Available.Currency != "USD" {
		t.Fatalf("response = %#v", payload)
	}
}

type stubWalletReader struct {
	snapshot wallet.Snapshot
}

func (s stubWalletReader) WalletByUser(context.Context, identity.UserID) (wallet.Snapshot, error) {
	return s.snapshot, nil
}

type walletSessionManager struct {
	user identity.User
}

func (s walletSessionManager) Start(context.Context, string) (identity.StartedSession, error) {
	return identity.StartedSession{}, nil
}

func (s walletSessionManager) Authenticate(context.Context, string) (identity.User, error) {
	return s.user, nil
}

func (s walletSessionManager) End(context.Context, string) error { return nil }
