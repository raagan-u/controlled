package identity_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/pay-pact/internal/identity"
)

func TestSessionLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.October, 23, 8, 0, 0, 0, time.UTC)
	user, err := identity.NewUser("user-1", "founder@example.com", "Raagan", now)
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}
	store := newMemorySessionStore(user)
	service := identity.NewSessionService(
		staticID{id: "session-1"},
		staticToken{token: "secret-token"},
		staticClock{now: now},
		store,
		24*time.Hour,
	)

	started, err := service.Start(t.Context(), " Founder@Example.com ")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.Token != "secret-token" || started.User.ID() != user.ID() || started.ExpiresAt != now.Add(24*time.Hour) {
		t.Fatalf("Start() = %+v", started)
	}
	authenticated, err := service.Authenticate(t.Context(), started.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if authenticated.ID() != user.ID() {
		t.Fatalf("Authenticate() user = %s, want %s", authenticated.ID(), user.ID())
	}
	if err := service.End(t.Context(), started.Token); err != nil {
		t.Fatalf("End() error = %v", err)
	}
	if _, err := service.Authenticate(t.Context(), started.Token); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("Authenticate() after End error = %v, want %v", err, identity.ErrInvalidCredentials)
	}
}

func TestSessionStartRejectsUnknownUser(t *testing.T) {
	t.Parallel()

	service := identity.NewSessionService(
		staticID{id: "session-1"},
		staticToken{token: "secret-token"},
		staticClock{now: time.Now()},
		newMemorySessionStore(identity.User{}),
		time.Hour,
	)
	if _, err := service.Start(t.Context(), "missing@example.com"); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("Start() error = %v, want %v", err, identity.ErrInvalidCredentials)
	}
}

type staticID struct{ id string }

func (s staticID) NewID() (string, error) { return s.id, nil }

type staticToken struct{ token string }

func (s staticToken) NewToken() (string, error) { return s.token, nil }

type staticClock struct{ now time.Time }

func (s staticClock) Now() time.Time { return s.now }

type storedSession struct {
	userID    identity.UserID
	expiresAt time.Time
	revoked   bool
}

type memorySessionStore struct {
	user     identity.User
	sessions map[[sha256.Size]byte]storedSession
}

func newMemorySessionStore(user identity.User) *memorySessionStore {
	return &memorySessionStore{user: user, sessions: make(map[[sha256.Size]byte]storedSession)}
}

func (s *memorySessionStore) UserByEmail(_ context.Context, email string) (identity.User, error) {
	if s.user.ID() == "" || s.user.Email() != email {
		return identity.User{}, identity.ErrInvalidCredentials
	}
	return s.user, nil
}

func (s *memorySessionStore) CreateSession(_ context.Context, session identity.Session) error {
	s.sessions[session.TokenHash()] = storedSession{userID: session.UserID(), expiresAt: session.ExpiresAt()}
	return nil
}

func (s *memorySessionStore) UserBySession(
	_ context.Context,
	hash [sha256.Size]byte,
	now time.Time,
) (identity.User, error) {
	session, ok := s.sessions[hash]
	if !ok || session.revoked || !session.expiresAt.After(now) {
		return identity.User{}, identity.ErrInvalidCredentials
	}
	return s.user, nil
}

func (s *memorySessionStore) RevokeSession(
	_ context.Context,
	hash [sha256.Size]byte,
	_ time.Time,
) error {
	session, ok := s.sessions[hash]
	if !ok || session.revoked {
		return identity.ErrSessionNotFound
	}
	session.revoked = true
	s.sessions[hash] = session
	return nil
}
