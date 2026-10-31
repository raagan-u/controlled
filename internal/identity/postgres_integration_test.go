package identity_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/platform"
	platformpostgres "github.com/raagan-u/controlled/internal/platform/postgres"
)

func TestPostgresSessionLifecycle(t *testing.T) {
	databaseURL := os.Getenv("CONTROLLED_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("CONTROLLED_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	if err := platformpostgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	ids := platform.NewUUIDGenerator()
	userID, err := ids.NewID()
	if err != nil {
		t.Fatalf("generate user ID: %v", err)
	}
	email := fmt.Sprintf("session-%s@example.test", userID)
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, created_at)
		VALUES ($1::text::uuid, $2, 'Session Test', $3)
	`, userID, email, time.Now().UTC()); err != nil {
		t.Fatalf("create test user: %v", err)
	}

	service := identity.NewSessionService(
		ids,
		platform.NewTokenGenerator(),
		platform.SystemClock{},
		identity.NewPostgresSessionStore(pool),
		time.Hour,
	)
	started, err := service.Start(ctx, email)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	authenticated, err := service.Authenticate(ctx, started.Token)
	if err != nil {
		t.Fatalf("authenticate session: %v", err)
	}
	if authenticated.ID() != identity.UserID(userID) {
		t.Fatalf("authenticated user = %s, want %s", authenticated.ID(), userID)
	}
	if err := service.End(ctx, started.Token); err != nil {
		t.Fatalf("end session: %v", err)
	}
	if _, err := service.Authenticate(ctx, started.Token); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("authenticate revoked session error = %v, want %v", err, identity.ErrInvalidCredentials)
	}
}

func TestPostgresUpdateDisplayNameClearsPlaceholder(t *testing.T) {
	databaseURL := os.Getenv("CONTROLLED_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("CONTROLLED_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := platformpostgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	userID, err := platform.NewUUIDGenerator().NewID()
	if err != nil {
		t.Fatalf("generate user ID: %v", err)
	}
	email := fmt.Sprintf("pending-%s@example.test", userID)
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, name_pending, created_at)
		VALUES ($1::text::uuid, $2, 'pending', true, $3)
	`, userID, email, time.Now().UTC()); err != nil {
		t.Fatalf("create test user: %v", err)
	}

	store := identity.NewPostgresSessionStore(pool)
	before, err := store.UserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("read user: %v", err)
	}
	if !before.NamePending() || before.DisplayName() != "pending" {
		t.Fatalf("before = %q pending=%t, want placeholder pending", before.DisplayName(), before.NamePending())
	}

	updated, err := store.UpdateDisplayName(ctx, identity.UserID(userID), "Ada Lovelace")
	if err != nil {
		t.Fatalf("update display name: %v", err)
	}
	if updated.DisplayName() != "Ada Lovelace" || updated.NamePending() {
		t.Fatalf("updated = %q pending=%t", updated.DisplayName(), updated.NamePending())
	}
	after, err := store.UserByEmail(ctx, email)
	if err != nil || after.DisplayName() != "Ada Lovelace" || after.NamePending() {
		t.Fatalf("after = %q pending=%t err=%v", after.DisplayName(), after.NamePending(), err)
	}
}
