package notification

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

type repositoryStub struct {
	registeredToken    string
	registeredPlatform string
	unregisteredToken  string
	tokens             []string
	removed            []string
	delivered          chan struct{}
}

func (r *repositoryStub) Register(_ context.Context, _ identity.UserID, token, platform string, _ time.Time) error {
	r.registeredToken, r.registeredPlatform = token, platform
	return nil
}

func (r *repositoryStub) Unregister(_ context.Context, _ identity.UserID, token string) error {
	r.unregisteredToken = token
	return nil
}

func (r *repositoryStub) Tokens(context.Context, identity.UserID) ([]string, error) {
	return r.tokens, nil
}

func (r *repositoryStub) RemoveTokens(_ context.Context, tokens []string) error {
	r.removed = append([]string(nil), tokens...)
	if r.delivered != nil {
		close(r.delivered)
	}
	return nil
}

type senderStub struct {
	tokens  []string
	message Message
	invalid []string
}

func (s *senderStub) Send(_ context.Context, tokens []string, message Message) ([]string, error) {
	s.tokens = append([]string(nil), tokens...)
	s.message = message
	return s.invalid, nil
}

func TestServiceRegistersAndUnregistersExpoDevice(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository, &senderStub{})
	userID := identity.UserID("9ca0be64-b1da-4b09-a11b-a43e59d6cc2a")
	token := "ExponentPushToken[test-token]"

	if err := service.Register(t.Context(), userID, "  "+token+"  ", "ANDROID"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if repository.registeredToken != token || repository.registeredPlatform != "android" {
		t.Fatalf("registered device = (%q, %q)", repository.registeredToken, repository.registeredPlatform)
	}
	if err := service.Unregister(t.Context(), userID, token); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	if repository.unregisteredToken != token {
		t.Fatalf("unregistered token = %q", repository.unregisteredToken)
	}
}

func TestServiceRejectsInvalidDevice(t *testing.T) {
	service := NewService(&repositoryStub{}, &senderStub{})
	userID := identity.UserID("9ca0be64-b1da-4b09-a11b-a43e59d6cc2a")

	if err := service.Register(t.Context(), userID, "not-an-expo-token", "android"); !errors.Is(err, ErrInvalidDevice) {
		t.Fatalf("Register() error = %v, want %v", err, ErrInvalidDevice)
	}
	if err := service.Register(t.Context(), userID, "ExpoPushToken[test-token]", "web"); !errors.Is(err, ErrInvalidDevice) {
		t.Fatalf("Register() error = %v, want %v", err, ErrInvalidDevice)
	}
}

func TestServiceDeliversAndRemovesInvalidTokens(t *testing.T) {
	delivered := make(chan struct{})
	repository := &repositoryStub{
		tokens:    []string{"ExpoPushToken[valid]", "ExpoPushToken[stale]"},
		delivered: delivered,
	}
	sender := &senderStub{invalid: []string{"ExpoPushToken[stale]"}}
	service := NewService(repository, sender)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go service.Run(ctx)

	message := Message{Title: "Approval needed", Body: "A transfer needs your decision.", Kind: KindAccountabilityRequest, EntityID: "request-1"}
	service.Notify(identity.UserID("9ca0be64-b1da-4b09-a11b-a43e59d6cc2a"), message)

	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("notification was not delivered")
	}
	if !reflect.DeepEqual(sender.tokens, repository.tokens) || sender.message != message {
		t.Fatalf("delivery = (%v, %#v)", sender.tokens, sender.message)
	}
	if !reflect.DeepEqual(repository.removed, sender.invalid) {
		t.Fatalf("removed tokens = %v, want %v", repository.removed, sender.invalid)
	}
}
