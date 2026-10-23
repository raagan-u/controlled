package walletops

import (
	"errors"
	"testing"
)

func TestPostgresRepositoryRequiresPool(t *testing.T) {
	t.Parallel()

	repository := NewPostgresRepository(nil, nil)
	if _, err := repository.CreateFunding(t.Context(), "key", FundingIntent{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("CreateFunding() error = %v, want %v", err, ErrUnavailable)
	}
	if _, err := repository.CreateTransfer(t.Context(), "key", Transfer{}, "journal"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("CreateTransfer() error = %v, want %v", err, ErrUnavailable)
	}
	if _, err := repository.ReserveWithdrawal(t.Context(), "key", Withdrawal{}, "journal"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ReserveWithdrawal() error = %v, want %v", err, ErrUnavailable)
	}
}

func TestOperationUUIDRejectsDomainIdentifiers(t *testing.T) {
	t.Parallel()

	if _, err := operationUUID("operation-1"); !errors.Is(err, ErrPersistenceID) {
		t.Fatalf("operationUUID() error = %v, want %v", err, ErrPersistenceID)
	}
}
