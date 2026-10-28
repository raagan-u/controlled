package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

func TestGoogleAuthenticationUsesLinkedIdentity(t *testing.T) {
	t.Parallel()

	user := googleTestUser(t)
	store := &googleIdentityStore{subjectUser: user}
	provisioner := &googleProvisioner{user: user}
	sessions := &googleSessions{started: googleStartedSession(user)}
	service := identity.NewGoogleAuthenticationService(
		googleVerifier{profile: googleProfile()}, store, provisioner, sessions,
	)

	started, err := service.Authenticate(context.Background(), "google-id-token")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if started.User.ID() != user.ID() || sessions.email != user.Email() {
		t.Fatalf("started session = %#v, email = %q", started, sessions.email)
	}
	if provisioner.calls != 0 || store.linkedSubject != "" {
		t.Fatalf("existing identity was provisioned or linked again")
	}
}

func TestGoogleAuthenticationProvisionsAndLinksNewUser(t *testing.T) {
	t.Parallel()

	user := googleTestUser(t)
	store := &googleIdentityStore{}
	provisioner := &googleProvisioner{user: user}
	sessions := &googleSessions{started: googleStartedSession(user)}
	service := identity.NewGoogleAuthenticationService(
		googleVerifier{profile: googleProfile()}, store, provisioner, sessions,
	)

	if _, err := service.Authenticate(context.Background(), "google-id-token"); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if provisioner.calls != 1 || store.linkedUserID != user.ID() ||
		store.linkedSubject != googleProfile().Subject || store.linkedEmail != googleProfile().Email {
		t.Fatalf("provision calls = %d, link = %q/%q/%q", provisioner.calls, store.linkedUserID, store.linkedSubject, store.linkedEmail)
	}
}

func TestGoogleAuthenticationRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	service := identity.NewGoogleAuthenticationService(
		googleVerifier{err: identity.ErrInvalidGoogleToken},
		&googleIdentityStore{},
		&googleProvisioner{},
		&googleSessions{},
	)
	_, err := service.Authenticate(context.Background(), "bad-token")
	if !errors.Is(err, identity.ErrInvalidGoogleToken) {
		t.Fatalf("Authenticate() error = %v", err)
	}
}

func googleProfile() identity.GoogleProfile {
	return identity.GoogleProfile{
		Subject: "google-subject-1", Email: "founder@example.com", DisplayName: "Raagan",
	}
}

func googleTestUser(t *testing.T) identity.User {
	t.Helper()
	user, err := identity.NewUser("user-1", "founder@example.com", "Raagan", time.Now())
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}
	return user
}

func googleStartedSession(user identity.User) identity.StartedSession {
	return identity.StartedSession{User: user, Token: "session-token", ExpiresAt: time.Now().Add(time.Hour)}
}

type googleVerifier struct {
	profile identity.GoogleProfile
	err     error
}

func (v googleVerifier) Verify(context.Context, string) (identity.GoogleProfile, error) {
	return v.profile, v.err
}

type googleIdentityStore struct {
	subjectUser   identity.User
	emailUser     identity.User
	linkedUserID  identity.UserID
	linkedSubject string
	linkedEmail   string
}

func (s *googleIdentityStore) UserByGoogleSubject(context.Context, string) (identity.User, error) {
	if s.subjectUser.ID() == "" {
		return identity.User{}, identity.ErrGoogleIdentityNotFound
	}
	return s.subjectUser, nil
}

func (s *googleIdentityStore) UserByEmail(context.Context, string) (identity.User, error) {
	if s.emailUser.ID() == "" {
		return identity.User{}, identity.ErrInvalidCredentials
	}
	return s.emailUser, nil
}

func (s *googleIdentityStore) LinkGoogleIdentity(
	_ context.Context,
	userID identity.UserID,
	subject string,
	email string,
) error {
	s.linkedUserID = userID
	s.linkedSubject = subject
	s.linkedEmail = email
	return nil
}

type googleProvisioner struct {
	user  identity.User
	calls int
}

func (p *googleProvisioner) ProvisionGoogleUser(context.Context, identity.GoogleProfile) (identity.User, error) {
	p.calls++
	return p.user, nil
}

type googleSessions struct {
	started identity.StartedSession
	email   string
}

func (s *googleSessions) Start(_ context.Context, email string) (identity.StartedSession, error) {
	s.email = email
	return s.started, nil
}
