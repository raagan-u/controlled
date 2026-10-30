package accountability

import (
	"errors"
	"testing"
)

func TestPostgresRepositoryRequiresPool(t *testing.T) {
	repository := NewPostgresRepository(nil)
	if _, err := repository.InvitationByID(t.Context(), "id"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("InvitationByID() error = %v, want unavailable", err)
	}
}

func TestAccountabilityUUIDRejectsDomainIdentifier(t *testing.T) {
	if _, err := accountabilityUUID("circle-1"); !errors.Is(err, ErrPersistenceID) {
		t.Fatalf("accountabilityUUID() error = %v, want persistence ID", err)
	}
}
