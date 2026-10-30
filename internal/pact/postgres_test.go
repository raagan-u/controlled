package pact

import (
	"errors"
	"testing"
)

func TestPostgresRepositoryRequiresPool(t *testing.T) {
	repository := NewPostgresRepository(nil)
	if _, err := repository.ByID(t.Context(), "owner", "pact"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ByID() error = %v, want unavailable", err)
	}
}

func TestPactUUIDRejectsDomainIdentifier(t *testing.T) {
	if _, err := pactUUID("pact-1"); !errors.Is(err, ErrPersistenceID) {
		t.Fatalf("pactUUID() error = %v, want persistence ID", err)
	}
}
