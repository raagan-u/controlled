// Package identity owns Controlled users and their authentication identity.
package identity

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrInvalidUserID      = errors.New("user ID is required")
	ErrInvalidEmail       = errors.New("a valid email address is required")
	ErrInvalidDisplayName = errors.New("display name is required")
	ErrInvalidCreatedAt   = errors.New("user creation time is required")
)

type UserID string

type User struct {
	id          UserID
	email       string
	displayName string
	createdAt   time.Time
}

func NewUser(id UserID, email, displayName string, createdAt time.Time) (User, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	normalizedName := strings.TrimSpace(displayName)
	if id == "" {
		return User{}, ErrInvalidUserID
	}
	if !validEmail(normalizedEmail) {
		return User{}, ErrInvalidEmail
	}
	if normalizedName == "" {
		return User{}, ErrInvalidDisplayName
	}
	if createdAt.IsZero() {
		return User{}, ErrInvalidCreatedAt
	}

	return User{
		id:          id,
		email:       normalizedEmail,
		displayName: normalizedName,
		createdAt:   createdAt.UTC(),
	}, nil
}

func (u User) ID() UserID           { return u.id }
func (u User) Email() string        { return u.email }
func (u User) DisplayName() string  { return u.displayName }
func (u User) CreatedAt() time.Time { return u.createdAt }

func validEmail(value string) bool {
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}
