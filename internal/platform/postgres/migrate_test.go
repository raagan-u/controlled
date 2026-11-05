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
	if len(found) != 19 {
		t.Fatalf("migration count = %d, want 19", len(found))
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
	if found[4].version != 5 || found[4].name != "000005_password_credentials.up.sql" || found[4].sql == "" {
		t.Fatalf("migration = %#v", found[4])
	}
	if found[5].version != 6 || found[5].name != "000006_accountability_relationships.up.sql" || found[5].sql == "" {
		t.Fatalf("migration = %#v", found[5])
	}
	if found[6].version != 7 || found[6].name != "000007_pacts.up.sql" || found[6].sql == "" {
		t.Fatalf("migration = %#v", found[6])
	}
	if found[7].version != 8 || found[7].name != "000008_pact_stakes.up.sql" || found[7].sql == "" {
		t.Fatalf("migration = %#v", found[7])
	}
	if found[8].version != 9 || found[8].name != "000009_pact_usage.up.sql" || found[8].sql == "" {
		t.Fatalf("migration = %#v", found[8])
	}
	if found[9].version != 10 || found[9].name != "000010_accountability_requests.up.sql" || found[9].sql == "" {
		t.Fatalf("migration = %#v", found[9])
	}
	if found[10].version != 11 || found[10].name != "000011_ctrld_ids.up.sql" || found[10].sql == "" {
		t.Fatalf("migration = %#v", found[10])
	}
	if found[11].version != 12 || found[11].name != "000012_payment_requests.up.sql" || found[11].sql == "" {
		t.Fatalf("migration = %#v", found[11])
	}
	if found[12].version != 13 || found[12].name != "000013_payment_request_lifecycle.up.sql" || found[12].sql == "" {
		t.Fatalf("migration = %#v", found[12])
	}
	if found[13].version != 14 || found[13].name != "000014_funding_net_amount.up.sql" || found[13].sql == "" {
		t.Fatalf("migration = %#v", found[13])
	}
	if found[14].version != 15 || found[14].name != "000015_user_name_pending.up.sql" || found[14].sql == "" {
		t.Fatalf("migration = %#v", found[14])
	}
	if found[15].version != 16 || found[15].name != "000016_push_devices.up.sql" || found[15].sql == "" {
		t.Fatalf("migration = %#v", found[15])
	}
	if found[16].version != 17 || found[16].name != "000017_transfer_remarks.up.sql" || found[16].sql == "" {
		t.Fatalf("migration = %#v", found[16])
	}
	if found[17].version != 18 || found[17].name != "000018_accountability_denial_reasons.up.sql" || found[17].sql == "" {
		t.Fatalf("migration = %#v", found[17])
	}
	if found[18].version != 19 || found[18].name != "000019_ai_transaction_assessments.up.sql" || found[18].sql == "" {
		t.Fatalf("migration = %#v", found[18])
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
