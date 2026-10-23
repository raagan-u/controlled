package identity

import (
	"errors"
	"testing"
)

func TestPostgresSessionStoreRequiresPool(t *testing.T) {
	t.Parallel()

	store := NewPostgresSessionStore(nil)
	if _, err := store.UserByEmail(t.Context(), "founder@example.com"); !errors.Is(err, ErrSessionStore) {
		t.Fatalf("UserByEmail() error = %v, want %v", err, ErrSessionStore)
	}
	if err := store.CreateSession(t.Context(), Session{}); !errors.Is(err, ErrSessionStore) {
		t.Fatalf("CreateSession() error = %v, want %v", err, ErrSessionStore)
	}
}

func TestIdentityUUIDRejectsDomainIdentifiers(t *testing.T) {
	t.Parallel()

	if _, err := identityUUID("session-1"); !errors.Is(err, ErrIdentityPersistence) {
		t.Fatalf("identityUUID() error = %v, want %v", err, ErrIdentityPersistence)
	}
}
