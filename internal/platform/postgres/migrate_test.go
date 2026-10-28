package postgres

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/raagan-u/controlled/migrations"
)

func TestDiscoverEmbeddedMigrations(t *testing.T) {
	t.Parallel()

	found, err := discoverMigrations(migrations.Files)
	if err != nil {
		t.Fatalf("discoverMigrations() error = %v", err)
	}
	if len(found) != 4 {
		t.Fatalf("migration count = %d, want 4", len(found))
	}
	if found[0].version != 1 || found[0].name != "000001_wallet_ledger.up.sql" || found[0].sql == "" {
		t.Fatalf("migration = %#v", found[0])
	}
	if found[1].version != 2 || found[1].name != "000002_sessions.up.sql" || found[1].sql == "" {
		t.Fatalf("migration = %#v", found[1])
	}
	if found[2].version != 3 || found[2].name != "000003_wallet_operations.up.sql" || found[2].sql == "" {
		t.Fatalf("migration = %#v", found[2])
	}
	if found[3].version != 4 || found[3].name != "000004_google_identities.up.sql" || found[3].sql == "" {
		t.Fatalf("migration = %#v", found[3])
	}
}

func TestMigrateRequiresPool(t *testing.T) {
	t.Parallel()

	if err := Migrate(t.Context(), nil); err == nil {
		t.Fatal("Migrate() error = nil")
	}
}

func TestDiscoverRejectsInvalidMigrationSet(t *testing.T) {
	t.Parallel()

	_, err := discoverMigrations(fstest.MapFS{})
	if !errors.Is(err, ErrInvalidMigration) {
		t.Fatalf("discoverMigrations() error = %v, want %v", err, ErrInvalidMigration)
	}
}
