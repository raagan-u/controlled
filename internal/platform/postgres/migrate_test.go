package postgres

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/raagan-u/pay-pact/migrations"
)

func TestDiscoverEmbeddedMigrations(t *testing.T) {
	t.Parallel()

	found, err := discoverMigrations(migrations.Files)
	if err != nil {
		t.Fatalf("discoverMigrations() error = %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("migration count = %d, want 1", len(found))
	}
	if found[0].version != 1 || found[0].name != "000001_wallet_ledger.up.sql" || found[0].sql == "" {
		t.Fatalf("migration = %#v", found[0])
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
