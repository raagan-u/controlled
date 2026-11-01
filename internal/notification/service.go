package notification

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

var (
	ErrInvalidDevice = errors.New("push device is invalid")
	ErrUnavailable   = errors.New("push notifications are unavailable")
)

type Repository interface {
	Register(context.Context, identity.UserID, string, string, time.Time) error
	Unregister(context.Context, identity.UserID, string) error
	Tokens(context.Context, identity.UserID) ([]string, error)
	RemoveTokens(context.Context, []string) error
}

type Sender interface {
	Send(context.Context, []string, Message) ([]string, error)
}

type delivery struct {
	recipient identity.UserID
	message   Message
}

type Service struct {
	repository Repository
	sender     Sender
	queue      chan delivery
}

func NewService(repository Repository, sender Sender) *Service {
	return &Service{repository: repository, sender: sender, queue: make(chan delivery, 128)}
}

func (s *Service) Register(ctx context.Context, userID identity.UserID, token, platform string) error {
	if s == nil || s.repository == nil {
		return ErrUnavailable
	}
	token = strings.TrimSpace(token)
	platform = strings.ToLower(strings.TrimSpace(platform))
	if userID == "" || !validExpoToken(token) || (platform != "android" && platform != "ios") {
		return ErrInvalidDevice
	}
	return s.repository.Register(ctx, userID, token, platform, time.Now().UTC())
}

func (s *Service) Unregister(ctx context.Context, userID identity.UserID, token string) error {
	if s == nil || s.repository == nil {
		return ErrUnavailable
	}
	token = strings.TrimSpace(token)
	if userID == "" || !validExpoToken(token) {
		return ErrInvalidDevice
	}
	return s.repository.Unregister(ctx, userID, token)
}

func (s *Service) Notify(recipient identity.UserID, message Message) {
	if s == nil || s.sender == nil || recipient == "" || strings.TrimSpace(message.Title) == "" ||
		strings.TrimSpace(message.Body) == "" || strings.TrimSpace(message.Kind) == "" || strings.TrimSpace(message.EntityID) == "" {
		return
	}
	select {
	case s.queue <- delivery{recipient: recipient, message: message}:
	default:
		slog.Warn("Push notification queue full", "recipient", recipient, "kind", message.Kind)
	}
}

func (s *Service) Run(ctx context.Context) {
	if s == nil || s.repository == nil || s.sender == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-s.queue:
			s.deliver(ctx, item)
		}
	}
}

func (s *Service) deliver(ctx context.Context, item delivery) {
	tokens, err := s.repository.Tokens(ctx, item.recipient)
	if err != nil || len(tokens) == 0 {
		if err != nil {
			slog.Error("Read push devices", "error", err, "recipient", item.recipient)
		}
		return
	}
	invalid, err := s.sender.Send(ctx, tokens, item.message)
	if err != nil {
		slog.Error("Send push notification", "error", err, "recipient", item.recipient, "kind", item.message.Kind)
		return
	}
	if len(invalid) > 0 {
		if err := s.repository.RemoveTokens(ctx, invalid); err != nil {
			slog.Error("Remove invalid push devices", "error", err)
		}
	}
}

func validExpoToken(token string) bool {
	return len(token) <= 512 && ((strings.HasPrefix(token, "ExponentPushToken[") || strings.HasPrefix(token, "ExpoPushToken[")) && strings.HasSuffix(token, "]"))
}
