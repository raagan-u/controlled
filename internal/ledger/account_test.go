package ledger_test

import (
	"errors"
	"testing"

	"github.com/raagan-u/controlled/internal/ledger"
)

func TestNewAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      ledger.AccountID
		ownerID ledger.OwnerID
		kind    ledger.AccountKind
		wantErr error
	}{
		{name: "available", id: "available-1", ownerID: "user-1", kind: ledger.AccountKindAvailable},
		{name: "custody asset", id: "custody-usd", kind: ledger.AccountKindCustodyAsset},
		{name: "stake vault", id: "stake-usd", kind: ledger.AccountKindStakeVault},
		{name: "wallet account requires owner", id: "available-1", kind: ledger.AccountKindAvailable, wantErr: ledger.ErrInvalidAccount},
		{name: "system account rejects owner", id: "custody-usd", ownerID: "user-1", kind: ledger.AccountKindCustodyAsset, wantErr: ledger.ErrInvalidAccount},
		{name: "invalid kind", id: "account-1", kind: "OTHER", wantErr: ledger.ErrInvalidAccount},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			account, err := ledger.NewAccount(test.id, test.ownerID, test.kind, "USD")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("NewAccount() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && !account.Valid() {
				t.Fatal("NewAccount() returned an invalid account")
			}
		})
	}
}
