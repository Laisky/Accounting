package storage

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestSQLiteStorageMigrateAppliesCoreSchema verifies embedded migrations build the SQLite schema.
func TestSQLiteStorageMigrateAppliesCoreSchema(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, DialectSQLite, db.Dialect())

	require.NoError(t, db.Migrate(ctx))
	require.NoError(t, db.Migrate(ctx))

	requireGooseVersion(t, db)
	requireStorageConformance(t, db)
	requireSQLiteEntryPlanUsesKeysetIndex(t, db)
}

// TestPostgresStorageMigrateAppliesCoreSchema verifies embedded migrations build the PostgreSQL schema when available.
func TestPostgresStorageMigrateAppliesCoreSchema(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		t.Skip("DATABASE_URL not set; skipping postgres storage integration test")
	}

	ctx := context.Background()
	db, err := Open(ctx, "postgres", databaseURL, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, DialectPostgres, db.Dialect())

	require.NoError(t, db.Migrate(ctx))
	require.NoError(t, db.Migrate(ctx))

	requireGooseVersion(t, db)
	requireStorageConformance(t, db)
	requirePostgresIndexExists(t, db, "entries_book_keyset_idx")
}

// TestRebindConvertsPostgresPlaceholders verifies query placeholder conversion is deterministic.
func TestRebindConvertsPostgresPlaceholders(t *testing.T) {
	require.Equal(t, "SELECT $1, $2, $3", Rebind(DialectPostgres, "SELECT ?, ?, ?"))
	require.Equal(t, "SELECT ?, ?", Rebind(DialectSQLite, "SELECT ?, ?"))
}

func requireGooseVersion(t *testing.T, db *DB) {
	t.Helper()

	var count int
	applied := any(true)
	if db.Dialect() == DialectSQLite {
		applied = 1
	}
	err := db.SQLDB().QueryRowContext(
		context.Background(),
		Rebind(db.Dialect(), `SELECT COUNT(*) FROM goose_db_version WHERE version_id IN (?, ?) AND is_applied = ?`),
		1,
		2,
		applied,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func requireStorageConformance(t *testing.T, db *DB) {
	t.Helper()

	ctx := context.Background()
	ownerID := "user-" + uuid.NewString()
	bookID := "book-" + uuid.NewString()
	groupID := "group-" + uuid.NewString()
	accountID := "account-" + uuid.NewString()
	categoryID := "category-" + uuid.NewString()
	entryID := "entry-" + uuid.NewString()
	email := ownerID + "@example.test"

	requireRollback(t, db)
	requireExec(t, db, `INSERT INTO users (id, email, status, password_hash) VALUES (?, ?, ?, ?)`,
		ownerID, email, "active", "hash")
	_, err := db.SQLDB().ExecContext(ctx, Rebind(db.Dialect(), `INSERT INTO users (id, email, status, password_hash) VALUES (?, ?, ?, ?)`),
		"user-"+uuid.NewString(), strings.ToUpper(email), "active", "hash")
	require.Error(t, err)

	requireExec(t, db, `INSERT INTO books (id, owner_user_id, name, reporting_currency) VALUES (?, ?, ?, ?)`,
		bookID, ownerID, "Household", "USD")
	requireExec(t, db, `INSERT INTO book_members (book_id, user_id, role, display_name) VALUES (?, ?, ?, ?)`,
		bookID, ownerID, "owner", "Owner")
	_, err = db.SQLDB().ExecContext(ctx, Rebind(db.Dialect(), `INSERT INTO book_members (book_id, user_id, role, display_name) VALUES (?, ?, ?, ?)`),
		bookID, ownerID, "owner", "Owner")
	require.Error(t, err)

	requireExec(t, db, `INSERT INTO account_groups (id, user_id, name) VALUES (?, ?, ?)`,
		groupID, ownerID, "Cash")
	requireExec(t, db, `INSERT INTO accounts (id, user_id, group_id, name, type, currency) VALUES (?, ?, ?, ?, ?, ?)`,
		accountID, ownerID, groupID, "Wallet", "cash", "USD")
	requireExec(t, db, `INSERT INTO account_shared_books (account_id, book_id) VALUES (?, ?)`,
		accountID, bookID)
	requireExec(t, db, `INSERT INTO categories (id, book_id, name, direction) VALUES (?, ?, ?, ?)`,
		categoryID, bookID, "Food", "expense")
	requireExec(t, db, `INSERT INTO entries (id, book_id, creator_user_id, type, account_id, category_id, amount_cents, transaction_currency, account_currency, book_reporting_currency, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entryID, bookID, ownerID, "expense", accountID, categoryID, 100, "USD", "USD", "USD", "2026-07-01T00:00:00Z")

	_, err = db.SQLDB().ExecContext(ctx, Rebind(db.Dialect(), `INSERT INTO entries (id, book_id, creator_user_id, type, account_id, category_id, amount_cents, transaction_currency, account_currency, book_reporting_currency, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		"entry-"+uuid.NewString(), bookID, ownerID, "expense", accountID, categoryID, 0, "USD", "USD", "USD", "2026-07-01T00:00:00Z")
	require.Error(t, err)

	_, err = db.SQLDB().ExecContext(ctx, Rebind(db.Dialect(), `INSERT INTO entries (id, book_id, creator_user_id, type, account_id, amount_cents, transaction_currency, account_currency, book_reporting_currency, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		"entry-"+uuid.NewString(), bookID, ownerID, "transfer", accountID, 100, "USD", "USD", "USD", "2026-07-01T00:00:00Z")
	require.Error(t, err)
}

func requireRollback(t *testing.T, db *DB) {
	t.Helper()

	rollbackID := "rollback-" + uuid.NewString()
	err := db.WithTx(context.Background(), func(tx DBTX) error {
		_, execErr := tx.ExecContext(context.Background(), Rebind(db.Dialect(), `INSERT INTO users (id, email, status, password_hash) VALUES (?, ?, ?, ?)`),
			rollbackID, rollbackID+"@example.test", "active", "hash")
		require.NoError(t, execErr)
		return errors.New("force rollback")
	})
	require.Error(t, err)

	var count int
	query := Rebind(db.Dialect(), `SELECT COUNT(*) FROM users WHERE id = ?`)
	require.NoError(t, db.SQLDB().QueryRowContext(context.Background(), query, rollbackID).Scan(&count))
	require.Zero(t, count)
}

func requireExec(t *testing.T, db *DB, query string, args ...any) {
	t.Helper()

	_, err := db.SQLDB().ExecContext(context.Background(), Rebind(db.Dialect(), query), args...)
	require.NoError(t, err)
}

func requireSQLiteEntryPlanUsesKeysetIndex(t *testing.T, db *DB) {
	t.Helper()

	rows, err := db.SQLDB().QueryContext(context.Background(), `EXPLAIN QUERY PLAN SELECT id FROM entries WHERE book_id = ? ORDER BY occurred_at DESC, id DESC LIMIT 10`, "unused")
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	var details []string
	for rows.Next() {
		var id int
		var parent int
		var unused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
		details = append(details, detail)
	}
	require.NoError(t, rows.Err())
	require.Contains(t, strings.Join(details, "\n"), "entries_book_keyset_idx")
}

func requirePostgresIndexExists(t *testing.T, db *DB, indexName string) {
	t.Helper()

	var count int
	err := db.SQLDB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM pg_indexes WHERE indexname = $1`, indexName).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
