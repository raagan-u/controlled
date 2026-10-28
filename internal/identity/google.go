package identity

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidGoogleToken     = errors.New("Google ID token is invalid")
	ErrGoogleIdentityNotFound = errors.New("Google identity was not found")
	ErrGoogleIdentityExists   = errors.New("Google identity already exists")
	ErrGoogleAuthentication   = errors.New("Google authentication is unavailable")
)

type GoogleProfile struct {
	Subject     string
	Email       string
	DisplayName string
}

type GoogleTokenVerifier interface {
	Verify(context.Context, string) (GoogleProfile, error)
}

type GoogleIdentityStore interface {
	UserByGoogleSubject(context.Context, string) (User, error)
	UserByEmail(context.Context, string) (User, error)
	LinkGoogleIdentity(context.Context, UserID, string, string) error
}

type GoogleUserProvisioner interface {
	ProvisionGoogleUser(context.Context, GoogleProfile) (User, error)
}

type SessionStarter interface {
	Start(context.Context, string) (StartedSession, error)
}

type GoogleAuthenticationService struct {
	verifier    GoogleTokenVerifier
	identities  GoogleIdentityStore
	provisioner GoogleUserProvisioner
	sessions    SessionStarter
}

func NewGoogleAuthenticationService(
	verifier GoogleTokenVerifier,
	identities GoogleIdentityStore,
	provisioner GoogleUserProvisioner,
	sessions SessionStarter,
) *GoogleAuthenticationService {
	return &GoogleAuthenticationService{
		verifier: verifier, identities: identities, provisioner: provisioner, sessions: sessions,
	}
}

func (s *GoogleAuthenticationService) Authenticate(ctx context.Context, rawToken string) (StartedSession, error) {
	if s == nil || s.verifier == nil || s.identities == nil || s.provisioner == nil || s.sessions == nil {
		return StartedSession{}, ErrGoogleAuthentication
	}
	if strings.TrimSpace(rawToken) == "" {
		return StartedSession{}, ErrInvalidGoogleToken
	}

	profile, err := s.verifier.Verify(ctx, rawToken)
	if err != nil {
		return StartedSession{}, err
	}
	user, err := s.identities.UserByGoogleSubject(ctx, profile.Subject)
	if err == nil {
		return s.sessions.Start(ctx, user.Email())
	}
	if !errors.Is(err, ErrGoogleIdentityNotFound) {
		return StartedSession{}, err
	}

	user, err = s.identities.UserByEmail(ctx, profile.Email)
	if errors.Is(err, ErrInvalidCredentials) {
		user, err = s.provisioner.ProvisionGoogleUser(ctx, profile)
	}
	if err != nil {
		return StartedSession{}, err
	}
	if err := s.identities.LinkGoogleIdentity(ctx, user.ID(), profile.Subject, profile.Email); err != nil {
		if !errors.Is(err, ErrGoogleIdentityExists) {
			return StartedSession{}, err
		}
		user, err = s.identities.UserByGoogleSubject(ctx, profile.Subject)
		if err != nil {
			return StartedSession{}, err
		}
	}
	return s.sessions.Start(ctx, user.Email())
}
