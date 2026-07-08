package imports

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// TestSQLRepositorySQLite exercises the relational import store round-trip on a migrated sqlite database.
func TestSQLRepositorySQLite(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))

	runSQLRepositoryContract(t, db)
}

// TestSQLRepositoryPostgres runs the same contract against postgres when DATABASE_URL is configured.
func TestSQLRepositoryPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		t.Skip("DATABASE_URL not set; skipping postgres import repository integration test")
	}

	ctx := context.Background()
	db, err := storage.Open(ctx, "postgres", databaseURL, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))

	runSQLRepositoryContract(t, db)
}

// runSQLRepositoryContract verifies SaveBatch/Batch/SaveBatchIfAbsent/BatchByHash against a migrated database.
func runSQLRepositoryContract(t *testing.T, db *storage.DB) {
	t.Helper()

	ctx := context.Background()
	repo, err := NewSQLRepository(db)
	require.NoError(t, err)

	userID := "user_" + strings.ReplaceAll(t.Name(), "/", "_")
	bookID := "book_" + strings.ReplaceAll(t.Name(), "/", "_")
	cleanupImportBatches(t, db, userID)
	seedImportUser(t, db, userID)
	seedImportBook(t, db, bookID, userID)

	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	batch := Batch{
		ID:            "batch_primary",
		UserID:        userID,
		Source:        "wacai",
		Filename:      "wacai.csv",
		ContentType:   "text/csv",
		SourceHash:    "hash_primary",
		ParserVersion: "wacai-v1",
		Status:        BatchStatusPreview,
		DetectedSchema: DetectedSchema{
			Columns: map[string]string{"amount": "金额", "occurredAt": "日期"},
			Missing: []string{"note"},
		},
		Detected: DetectedValues{
			Books:      []string{"Household"},
			Accounts:   []string{"Cash", "Card"},
			Currencies: []string{"CNY"},
		},
		Rows: []PreviewRow{
			{
				RowNumber:  1,
				Raw:        map[string]string{"金额": "12.50", "日期": "2026-07-01"},
				Type:       "expense",
				SourceType: "支出",
				OccurredAt: "2026-07-01",
				Amount:     "12.50",
				Currency:   "CNY",
				Account:    "Cash",
				Category:   "Food",
				Tags:       []string{"lunch"},
				Warnings:   []string{"low confidence"},
			},
			{
				RowNumber:          2,
				Raw:                map[string]string{"金额": "99.00", "日期": "2026-07-02"},
				Type:               "transfer",
				OccurredAt:         "2026-07-02",
				Amount:             "99.00",
				Currency:           "CNY",
				Account:            "Card",
				DestinationAccount: "Cash",
				Errors:             []string{"missing category"},
			},
			{
				RowNumber: 3,
				Raw:       map[string]string{"金额": "5.00", "日期": "2026-07-03"},
				Amount:    "5.00",
			},
		},
		ErrorCount:   1,
		WarningCount: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	saved, err := repo.SaveBatch(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, batch.ID, saved.ID)

	loaded, err := repo.Batch(ctx, userID, batch.ID)
	require.NoError(t, err)
	require.Equal(t, saved, loaded)
	require.Len(t, loaded.Rows, 3)

	// Ownership is enforced: another user cannot read the batch.
	_, err = repo.Batch(ctx, "someone_else", batch.ID)
	require.ErrorIs(t, err, ErrNotFound)
	// Unknown ids surface ErrNotFound.
	_, err = repo.Batch(ctx, userID, "batch_missing")
	require.ErrorIs(t, err, ErrNotFound)

	// SaveBatch upserts: replacing rows and setting applied metadata round-trips.
	appliedAt := now.Add(time.Hour)
	updated := saved
	updated.Status = BatchStatusApplied
	updated.AppliedBookID = bookID
	updated.AppliedEntryIDs = []string{"entry_1", "entry_2"}
	updated.AppliedSkippedRows = []AppliedSkippedRow{{RowNumber: 3, Reason: "missing amount"}}
	updated.AppliedAt = &appliedAt
	updated.UpdatedAt = now.Add(2 * time.Hour)
	updated.Rows = []PreviewRow{{
		RowNumber: 1,
		Raw:       map[string]string{"金额": "12.50"},
		Amount:    "12.50",
		Currency:  "CNY",
	}}
	resaved, err := repo.SaveBatch(ctx, updated)
	require.NoError(t, err)
	reloaded, err := repo.Batch(ctx, userID, batch.ID)
	require.NoError(t, err)
	require.Equal(t, resaved, reloaded)
	require.Len(t, reloaded.Rows, 1)
	require.Equal(t, BatchStatusApplied, reloaded.Status)
	require.Equal(t, bookID, reloaded.AppliedBookID)
	require.Equal(t, []string{"entry_1", "entry_2"}, reloaded.AppliedEntryIDs)
	require.NotNil(t, reloaded.AppliedAt)
	require.True(t, appliedAt.Equal(*reloaded.AppliedAt))

	// SaveBatchIfAbsent stores a fresh batch the first time.
	fresh := Batch{
		ID:            "batch_ifabsent",
		UserID:        userID,
		Source:        "wacai",
		Filename:      "again.csv",
		ContentType:   "text/csv",
		SourceHash:    "hash_ifabsent",
		ParserVersion: "wacai-v1",
		Status:        BatchStatusPreview,
		Rows: []PreviewRow{{
			RowNumber: 1,
			Raw:       map[string]string{"金额": "1.00"},
			Amount:    "1.00",
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	stored, created, err := repo.SaveBatchIfAbsent(ctx, fresh)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, fresh.ID, stored.ID)

	// A second call with the same owner/source/hash returns the existing batch untouched.
	duplicate := fresh
	duplicate.ID = "batch_ifabsent_dup"
	duplicate.Filename = "different.csv"
	duplicate.Rows = []PreviewRow{{RowNumber: 7, Raw: map[string]string{"x": "y"}}}
	again, created, err := repo.SaveBatchIfAbsent(ctx, duplicate)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, stored, again)
	require.Equal(t, "batch_ifabsent", again.ID)

	// BatchByHash resolves by owner/source/hash.
	byHash, err := repo.BatchByHash(ctx, userID, "wacai", "hash_ifabsent")
	require.NoError(t, err)
	require.Equal(t, stored, byHash)

	_, err = repo.BatchByHash(ctx, userID, "wacai", "hash_unknown")
	require.ErrorIs(t, err, ErrNotFound)
}

// cleanupImportBatches removes prior-run batches for a user so fixed ids are reusable on shared postgres.
func cleanupImportBatches(t *testing.T, db *storage.DB, userID string) {
	t.Helper()
	// import_rows references import_batches with ON DELETE CASCADE.
	_, err := db.SQLDB().ExecContext(context.Background(),
		storage.Rebind(db.Dialect(), `DELETE FROM import_batches WHERE user_id = ?`), userID)
	require.NoError(t, err)
}

func seedImportUser(t *testing.T, db *storage.DB, userID string) {
	t.Helper()
	_, err := db.SQLDB().ExecContext(context.Background(),
		storage.Rebind(db.Dialect(), `INSERT INTO users (id, email, status, password_hash) VALUES (?, ?, ?, ?)
			ON CONFLICT (id) DO NOTHING`),
		userID, userID+"@example.test", "active", "hash")
	require.NoError(t, err)
}

func seedImportBook(t *testing.T, db *storage.DB, bookID string, ownerID string) {
	t.Helper()
	_, err := db.SQLDB().ExecContext(context.Background(),
		storage.Rebind(db.Dialect(), `INSERT INTO books (id, owner_user_id, name, reporting_currency) VALUES (?, ?, ?, ?)
			ON CONFLICT (id) DO NOTHING`),
		bookID, ownerID, "Household", "CNY")
	require.NoError(t, err)
}
