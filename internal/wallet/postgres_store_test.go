package wallet

import (
	"errors"
	"testing"
)

func TestPostgresProvisioningStoreRequiresPool(t *testing.T) {
	t.Parallel()

	store := NewPostgresProvisioningStore(nil)
	if _, _, err := store.Create(t.Context(), "key", Provisioning{}); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("Create() error = %v, want %v", err, ErrStoreUnavailable)
	}
	if _, err := store.ByUser(t.Context(), "user"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("ByUser() error = %v, want %v", err, ErrStoreUnavailable)
	}
	if _, err := store.WalletByUser(t.Context(), "user"); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("WalletByUser() error = %v, want %v", err, ErrStoreUnavailable)
	}
}

func TestWalletUUIDRejectsDomainIdentifiers(t *testing.T) {
	t.Parallel()

	if _, err := walletUUID("user-1"); !errors.Is(err, ErrPersistenceID) {
		t.Fatalf("walletUUID() error = %v, want %v", err, ErrPersistenceID)
	}
}
