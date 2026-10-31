package identity_test

import (
	"context"
	"errors"
	"testing"

	"github.com/raagan-u/controlled/internal/identity"
)

func TestPasswordRegistrationCreatesCredentialAndSession(t *testing.T) {
	t.Parallel()

	user := googleTestUser(t)
	store := &passwordStore{}
	provisioner := &passwordProvisioner{user: user}
	sessions := &googleSessions{started: googleStartedSession(user)}
	service := identity.NewPasswordAuthenticationService(
		store, provisioner, passwordHasher{}, sessions,
	)

	started, err := service.Register(context.Background(), " Founder@Example.com ", "plain password", "Founder Name")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if started.User.ID() != user.ID() || provisioner.email != "founder@example.com" {
		t.Fatalf("started = %#v, provisioned email = %q", started, provisioner.email)
	}
	if provisioner.displayName != "Founder Name" {
		t.Fatalf("provisioned display name = %q", provisioner.displayName)
	}
	if store.createdUserID != user.ID() || store.createdHash != "hashed:plain password" {
		t.Fatalf("credential = %q/%q", store.createdUserID, store.createdHash)
	}
}

func TestPasswordRegistrationRejectsExistingEmail(t *testing.T) {
	t.Parallel()

	user := googleTestUser(t)
	service := identity.NewPasswordAuthenticationService(
		&passwordStore{emailUser: user},
		&passwordProvisioner{},
		passwordHasher{},
		&googleSessions{},
	)
	_, err := service.Register(context.Background(), user.Email(), "password", "Founder")
	if !errors.Is(err, identity.ErrEmailAlreadyRegistered) {
		t.Fatalf("Register() error = %v", err)
	}
}

func TestPasswordLoginStartsSession(t *testing.T) {
	t.Parallel()

	user := googleTestUser(t)
	sessions := &googleSessions{started: googleStartedSession(user)}
	service := identity.NewPasswordAuthenticationService(
		&passwordStore{credentialUser: user, passwordHash: "hashed:correct"},
		&passwordProvisioner{},
		passwordHasher{},
		sessions,
	)
	started, err := service.Login(context.Background(), user.Email(), "correct")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if started.User.ID() != user.ID() || sessions.email != user.Email() {
		t.Fatalf("started = %#v, session email = %q", started, sessions.email)
	}
}

func TestPasswordLoginRejectsWrongPassword(t *testing.T) {
	t.Parallel()

	user := googleTestUser(t)
	service := identity.NewPasswordAuthenticationService(
		&passwordStore{credentialUser: user, passwordHash: "hashed:correct"},
		&passwordProvisioner{},
		passwordHasher{},
		&googleSessions{},
	)
	_, err := service.Login(context.Background(), user.Email(), "wrong")
	if !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v", err)
	}
}

type passwordStore struct {
	emailUser      identity.User
	credentialUser identity.User
	passwordHash   string
	createdUserID  identity.UserID
	createdHash    string
}

func (s *passwordStore) UserByEmail(context.Context, string) (identity.User, error) {
	if s.emailUser.ID() == "" {
		return identity.User{}, identity.ErrInvalidCredentials
	}
	return s.emailUser, nil
}

func (s *passwordStore) UserAndPasswordHashByEmail(context.Context, string) (identity.User, string, error) {
	if s.credentialUser.ID() == "" {
		return identity.User{}, "", identity.ErrInvalidCredentials
	}
	return s.credentialUser, s.passwordHash, nil
}

func (s *passwordStore) CreatePasswordCredential(
	_ context.Context,
	userID identity.UserID,
	hash string,
) error {
	s.createdUserID = userID
	s.createdHash = hash
	return nil
}

type passwordProvisioner struct {
	user        identity.User
	email       string
	displayName string
}

func (p *passwordProvisioner) ProvisionPasswordUser(
	_ context.Context,
	email string,
	displayName string,
) (identity.User, error) {
	p.email = email
	p.displayName = displayName
	return p.user, nil
}

type passwordHasher struct{}

func (passwordHasher) Hash(password string) (string, error) {
	return "hashed:" + password, nil
}

func (passwordHasher) Matches(hash string, password string) bool {
	return hash == "hashed:"+password
}
