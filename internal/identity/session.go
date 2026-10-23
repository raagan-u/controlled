package identity

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidSession      = errors.New("session is invalid")
	ErrInvalidCredentials  = errors.New("credentials are invalid")
	ErrSessionNotFound     = errors.New("session was not found")
	ErrSessionUnavailable  = errors.New("session dependencies are unavailable")
	ErrInvalidSessionToken = errors.New("session token is required")
	ErrSessionStore        = errors.New("session store is unavailable")
	ErrIdentityPersistence = errors.New("identity persistence requires UUID identifiers")
)

type SessionID string

type Session struct {
	id        SessionID
	userID    UserID
	tokenHash [sha256.Size]byte
	createdAt time.Time
	expiresAt time.Time
}

func NewSession(
	id SessionID,
	userID UserID,
	tokenHash [sha256.Size]byte,
	createdAt time.Time,
	expiresAt time.Time,
) (Session, error) {
	if id == "" || userID == "" || tokenHash == ([sha256.Size]byte{}) ||
		createdAt.IsZero() || !expiresAt.After(createdAt) {
		return Session{}, ErrInvalidSession
	}
	return Session{
		id:        id,
		userID:    userID,
		tokenHash: tokenHash,
		createdAt: createdAt.UTC(),
		expiresAt: expiresAt.UTC(),
	}, nil
}

func (s Session) ID() SessionID                { return s.id }
func (s Session) UserID() UserID               { return s.userID }
func (s Session) TokenHash() [sha256.Size]byte { return s.tokenHash }
func (s Session) CreatedAt() time.Time         { return s.createdAt }
func (s Session) ExpiresAt() time.Time         { return s.expiresAt }

type SessionIDGenerator interface {
	NewID() (string, error)
}

type SessionTokenGenerator interface {
	NewToken() (string, error)
}

type SessionClock interface {
	Now() time.Time
}

type SessionStore interface {
	UserByEmail(context.Context, string) (User, error)
	CreateSession(context.Context, Session) error
	UserBySession(context.Context, [sha256.Size]byte, time.Time) (User, error)
	RevokeSession(context.Context, [sha256.Size]byte, time.Time) error
}

type SessionService struct {
	ids    SessionIDGenerator
	tokens SessionTokenGenerator
	clock  SessionClock
	store  SessionStore
	ttl    time.Duration
}

func NewSessionService(
	ids SessionIDGenerator,
	tokens SessionTokenGenerator,
	clock SessionClock,
	store SessionStore,
	ttl time.Duration,
) *SessionService {
	return &SessionService{ids: ids, tokens: tokens, clock: clock, store: store, ttl: ttl}
}

type StartedSession struct {
	User      User
	Token     string
	ExpiresAt time.Time
}

func (s *SessionService) Start(ctx context.Context, email string) (StartedSession, error) {
	if s == nil || s.ids == nil || s.tokens == nil || s.clock == nil || s.store == nil || s.ttl <= 0 {
		return StartedSession{}, ErrSessionUnavailable
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if !validEmail(normalizedEmail) {
		return StartedSession{}, ErrInvalidEmail
	}
	user, err := s.store.UserByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return StartedSession{}, err
		}
		return StartedSession{}, err
	}
	id, err := s.ids.NewID()
	if err != nil {
		return StartedSession{}, err
	}
	token, err := s.tokens.NewToken()
	if err != nil {
		return StartedSession{}, err
	}
	now := s.clock.Now().UTC()
	expiresAt := now.Add(s.ttl)
	session, err := NewSession(SessionID(id), user.ID(), sha256.Sum256([]byte(token)), now, expiresAt)
	if err != nil {
		return StartedSession{}, err
	}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return StartedSession{}, err
	}
	return StartedSession{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *SessionService) Authenticate(ctx context.Context, token string) (User, error) {
	if s == nil || s.clock == nil || s.store == nil {
		return User{}, ErrSessionUnavailable
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return User{}, ErrInvalidSessionToken
	}
	return s.store.UserBySession(ctx, sha256.Sum256([]byte(token)), s.clock.Now().UTC())
}

func (s *SessionService) End(ctx context.Context, token string) error {
	if s == nil || s.clock == nil || s.store == nil {
		return ErrSessionUnavailable
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrInvalidSessionToken
	}
	return s.store.RevokeSession(ctx, sha256.Sum256([]byte(token)), s.clock.Now().UTC())
}
