package wallet_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/money"
	"github.com/raagan-u/controlled/internal/wallet"
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
		Available: zero, Pending: zero, Staked: zero, CommitmentReserve: zero,
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

func TestWalletTransactionsHandler(t *testing.T) {
	t.Parallel()

	user, err := identity.NewUser("user-1", "founder@example.com", "Raagan", time.Now())
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}
	amount, err := money.New(2500, "USD")
	if err != nil {
		t.Fatalf("money.New() error = %v", err)
	}
	reader := stubWalletReader{transactions: []wallet.Transaction{{
		ID:          "journal-1",
		Reference:   ledger.Reference{Type: "FUNDING", ID: "funding-1"},
		Description: "Wallet funding",
		OccurredAt:  time.Now().UTC(),
		PostedAt:    time.Now().UTC(),
		Entries: []wallet.TransactionEntry{{
			AccountID: "available-1", AccountKind: ledger.AccountKindAvailable,
			Side: ledger.SideCredit, Amount: amount,
		}},
	}}}
	handler := wallet.NewReadHandler(reader)
	protected := identity.RequireSession(walletSessionManager{user: user}, http.HandlerFunc(handler.Transactions))
	request := httptest.NewRequest(http.MethodGet, "/v1/wallet/transactions", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Transactions []struct {
			ID        string `json:"id"`
			Reference struct {
				Type string `json:"type"`
			} `json:"reference"`
			Entries []struct {
				Amount struct {
					AmountMinor int64 `json:"amountMinor"`
				} `json:"amount"`
			} `json:"entries"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Transactions) != 1 || payload.Transactions[0].ID != "journal-1" ||
		payload.Transactions[0].Reference.Type != "FUNDING" ||
		payload.Transactions[0].Entries[0].Amount.AmountMinor != 2500 {
		t.Fatalf("response = %#v", payload)
	}
}

func TestWalletTransactionHandler(t *testing.T) {
	t.Parallel()
	user, err := identity.NewUser("user-1", "founder@example.com", "Raagan", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	amount, _ := money.New(900, "USD")
	reader := stubWalletReader{transaction: wallet.Transaction{
		ID: "journal-1", Reference: ledger.Reference{Type: "INTERNAL_TRANSFER", ID: "transfer-1"},
		Description: "Controlled internal transfer", OccurredAt: time.Now(), PostedAt: time.Now(),
		Entries: []wallet.TransactionEntry{{AccountID: "available-1", AccountKind: ledger.AccountKindAvailable, Side: ledger.SideDebit, Amount: amount}},
		Transfer: &wallet.TransferDetails{
			Sender:    wallet.TransactionParty{ID: "user-1", CtrldID: "ctrld00001", DisplayName: "Raagan", Email: "founder@example.com"},
			Recipient: wallet.TransactionParty{ID: "user-2", CtrldID: "ctrld00002", DisplayName: "Friend", Email: "friend@example.com"},
			Remark:    "Dinner", PaymentRequestID: "request-1",
		},
		Approval: &wallet.ApprovalDetails{
			RequestID: "approval-1", Reason: "Family emergency", Classification: "EMERGENCY",
			ApprovedBy: wallet.TransactionParty{ID: "user-3", CtrldID: "ctrld00003", DisplayName: "Partner", Email: "partner@example.com"},
		},
	}}
	handler := wallet.NewReadHandler(reader)
	protected := identity.RequireSession(walletSessionManager{user: user}, http.HandlerFunc(handler.Transaction))
	request := httptest.NewRequest(http.MethodGet, "/v1/wallet/transactions/journal-1", nil)
	request.SetPathValue("id", "journal-1")
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Transaction struct {
			ID              string `json:"id"`
			ApprovalDetails struct {
				Classification string `json:"classification"`
				Reason         string `json:"reason"`
				ApprovedBy     struct {
					DisplayName string `json:"displayName"`
				} `json:"approvedBy"`
			} `json:"approvalDetails"`
			TransferDetails struct {
				Remark string `json:"remark"`
				Sender struct {
					DisplayName string `json:"displayName"`
				} `json:"sender"`
			} `json:"transferDetails"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Transaction.ID != "journal-1" {
		t.Fatalf("response = %s, err = %v", response.Body.String(), err)
	}
	if payload.Transaction.TransferDetails.Remark != "Dinner" || payload.Transaction.TransferDetails.Sender.DisplayName != "Raagan" {
		t.Fatalf("transfer details = %#v", payload.Transaction.TransferDetails)
	}
	if payload.Transaction.ApprovalDetails.Classification != "EMERGENCY" || payload.Transaction.ApprovalDetails.Reason != "Family emergency" || payload.Transaction.ApprovalDetails.ApprovedBy.DisplayName != "Partner" {
		t.Fatalf("approval details = %#v", payload.Transaction.ApprovalDetails)
	}
}

type stubWalletReader struct {
	snapshot     wallet.Snapshot
	transactions []wallet.Transaction
	transaction  wallet.Transaction
}

func (s stubWalletReader) TransactionsByUser(context.Context, identity.UserID, int) ([]wallet.Transaction, error) {
	return s.transactions, nil
}

func (s stubWalletReader) TransactionByUser(context.Context, identity.UserID, ledger.JournalID) (wallet.Transaction, error) {
	return s.transaction, nil
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
