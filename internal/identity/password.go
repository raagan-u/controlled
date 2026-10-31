package identity

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const maxBcryptPasswordBytes = 72

var (
	ErrInvalidPassword          = errors.New("password is invalid")
	ErrEmailAlreadyRegistered   = errors.New("email is already registered")
	ErrPasswordCredentialExists = errors.New("password credential already exists")
	ErrPasswordAuthentication   = errors.New("password authentication is unavailable")
)

type PasswordCredentialStore interface {
	UserByEmail(context.Context, string) (User, error)
	UserAndPasswordHashByEmail(context.Context, string) (User, string, error)
	CreatePasswordCredential(context.Context, UserID, string) error
}

type PasswordUserProvisioner interface {
	ProvisionPasswordUser(context.Context, string, string) (User, error)
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Matches(string, string) bool
}

type BcryptPasswordHasher struct{}

func (BcryptPasswordHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (BcryptPasswordHasher) Matches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type PasswordAuthenticationService struct {
	store       PasswordCredentialStore
	provisioner PasswordUserProvisioner
	hasher      PasswordHasher
	sessions    SessionStarter
}

func NewPasswordAuthenticationService(
	store PasswordCredentialStore,
	provisioner PasswordUserProvisioner,
	hasher PasswordHasher,
	sessions SessionStarter,
) *PasswordAuthenticationService {
	return &PasswordAuthenticationService{
		store: store, provisioner: provisioner, hasher: hasher, sessions: sessions,
	}
}

func (s *PasswordAuthenticationService) Register(
	ctx context.Context,
	email string,
	password string,
	displayName string,
) (StartedSession, error) {
	if !s.available() {
		return StartedSession{}, ErrPasswordAuthentication
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) {
		return StartedSession{}, ErrInvalidEmail
	}
	if !validPassword(password) {
		return StartedSession{}, ErrInvalidPassword
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return StartedSession{}, ErrInvalidDisplayName
	}
	if _, err := s.store.UserByEmail(ctx, email); err == nil {
		return StartedSession{}, ErrEmailAlreadyRegistered
	} else if !errors.Is(err, ErrInvalidCredentials) {
		return StartedSession{}, err
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return StartedSession{}, err
	}
	user, err := s.provisioner.ProvisionPasswordUser(ctx, email, displayName)
	if err != nil {
		return StartedSession{}, err
	}
	if err := s.store.CreatePasswordCredential(ctx, user.ID(), hash); err != nil {
		return StartedSession{}, err
	}
	return s.sessions.Start(ctx, user.Email())
}

func (s *PasswordAuthenticationService) Login(
	ctx context.Context,
	email string,
	password string,
) (StartedSession, error) {
	if !s.available() {
		return StartedSession{}, ErrPasswordAuthentication
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) || !validPassword(password) {
		return StartedSession{}, ErrInvalidCredentials
	}
	user, hash, err := s.store.UserAndPasswordHashByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return StartedSession{}, ErrInvalidCredentials
		}
		return StartedSession{}, err
	}
	if !s.hasher.Matches(hash, password) {
		return StartedSession{}, ErrInvalidCredentials
	}
	return s.sessions.Start(ctx, user.Email())
}

func (s *PasswordAuthenticationService) available() bool {
	return s != nil && s.store != nil && s.provisioner != nil && s.hasher != nil && s.sessions != nil
}

func validPassword(password string) bool {
	return strings.TrimSpace(password) != "" && len([]byte(password)) <= maxBcryptPasswordBytes
}
