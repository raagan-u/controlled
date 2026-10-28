package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/migrations"
)

var (
	ErrInvalidMigration = errors.New("migration filename or content is invalid")
	ErrMigrationChanged = errors.New("an applied migration has changed")
)

const migrationLockID int64 = 6_207_971_675_274_141_828

type migration struct {
	version  int64
	name     string
	sql      string
	checksum [sha256.Size]byte
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("PostgreSQL pool is required")
	}

	all, err := discoverMigrations(migrations.Files)
	if err != nil {
		return err
	}
	connection, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer connection.Release()

	if _, err := connection.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer unlockMigrations(connection)

	if _, err := connection.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint PRIMARY KEY,
			name text NOT NULL,
			checksum bytea NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}

	applied, err := loadAppliedMigrations(ctx, connection)
	if err != nil {
		return err
	}
	for _, candidate := range all {
		if checksum, exists := applied[candidate.version]; exists {
			if checksum != candidate.checksum {
				return fmt.Errorf("%w: %s", ErrMigrationChanged, candidate.name)
			}
			continue
		}
		if err := applyMigration(ctx, connection, candidate); err != nil {
			return err
		}
	}
	return nil
}

func discoverMigrations(files fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	var found []migration
	versions := make(map[int64]struct{})
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		separator := strings.IndexByte(name, '_')
		if separator <= 0 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMigration, name)
		}
		version, err := strconv.ParseInt(name[:separator], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMigration, name)
		}
		if _, duplicate := versions[version]; duplicate {
			return nil, fmt.Errorf("%w: duplicate version %d", ErrInvalidMigration, version)
		}
		contents, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		sql := strings.TrimSpace(string(contents))
		if sql == "" {
			return nil, fmt.Errorf("%w: %s is empty", ErrInvalidMigration, name)
		}

		versions[version] = struct{}{}
		found = append(found, migration{
			version:  version,
			name:     name,
			sql:      sql,
			checksum: sha256.Sum256(contents),
		})
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("%w: no up migrations", ErrInvalidMigration)
	}
	sort.Slice(found, func(left, right int) bool {
		return found[left].version < found[right].version
	})
	return found, nil
}

func loadAppliedMigrations(ctx context.Context, connection *pgxpool.Conn) (map[int64][sha256.Size]byte, error) {
	rows, err := connection.Query(ctx, "SELECT version, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read migration history: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64][sha256.Size]byte)
	for rows.Next() {
		var version int64
		var rawChecksum []byte
		if err := rows.Scan(&version, &rawChecksum); err != nil {
			return nil, fmt.Errorf("scan migration history: %w", err)
		}
		if len(rawChecksum) != sha256.Size {
			return nil, fmt.Errorf("%w: version %d has an invalid checksum", ErrMigrationChanged, version)
		}
		var checksum [sha256.Size]byte
		copy(checksum[:], rawChecksum)
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration history: %w", err)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, connection *pgxpool.Conn, candidate migration) error {
	transaction, err := connection.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", candidate.name, err)
	}
	defer func() {
		_ = transaction.Rollback(context.Background())
	}()

	if _, err := transaction.Exec(ctx, candidate.sql); err != nil {
		return fmt.Errorf("apply migration %s: %w", candidate.name, err)
	}
	if _, err := transaction.Exec(
		ctx,
		"INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)",
		candidate.version,
		candidate.name,
		candidate.checksum[:],
	); err != nil {
		return fmt.Errorf("record migration %s: %w", candidate.name, err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", candidate.name, err)
	}
	return nil
}

func unlockMigrations(connection *pgxpool.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = connection.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLockID)
}
