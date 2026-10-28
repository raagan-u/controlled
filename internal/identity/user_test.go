package identity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

func TestNewUserNormalizesInput(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.October, 22, 12, 0, 0, 0, time.FixedZone("IST", 5*60*60+30*60))
	user, err := identity.NewUser("user-1", "  Founder@Example.COM ", "  Raagan  ", createdAt)
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}
	if user.Email() != "founder@example.com" {
		t.Fatalf("Email() = %q", user.Email())
	}
	if user.DisplayName() != "Raagan" {
		t.Fatalf("DisplayName() = %q", user.DisplayName())
	}
	if user.CreatedAt().Location() != time.UTC {
		t.Fatalf("CreatedAt() location = %v, want UTC", user.CreatedAt().Location())
	}
}

func TestNewUserRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name        string
		id          identity.UserID
		email       string
		displayName string
		createdAt   time.Time
		wantErr     error
	}{
		{name: "missing ID", email: "founder@example.com", displayName: "Raagan", createdAt: now, wantErr: identity.ErrInvalidUserID},
		{name: "invalid email", id: "user-1", email: "not-an-email", displayName: "Raagan", createdAt: now, wantErr: identity.ErrInvalidEmail},
		{name: "missing name", id: "user-1", email: "founder@example.com", createdAt: now, wantErr: identity.ErrInvalidDisplayName},
		{name: "missing time", id: "user-1", email: "founder@example.com", displayName: "Raagan", wantErr: identity.ErrInvalidCreatedAt},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := identity.NewUser(test.id, test.email, test.displayName, test.createdAt)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("NewUser() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
