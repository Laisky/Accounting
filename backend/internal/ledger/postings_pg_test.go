package ledger_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/storage"
)

// TestSQLRepositoryPostgresPostingsAndReconcile verifies the posting write path and reconciliation
// against a migrated PostgreSQL database when DATABASE_URL is configured. It lives in the external
// ledger_test package so its DATABASE_URL taint stays out of the ledger package's SQL analysis.
func TestSQLRepositoryPostgresPostingsAndReconcile(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping postgres postings integration test")
	}

	ctx := context.Background()
	db, err := storage.Open(ctx, "postgres", databaseURL, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))
	repo, err := ledger.NewSQLRepository(db)
	require.NoError(t, err)

	suffix := uuid.NewString()
	userID := "user-" + suffix
	bookID := "book-" + suffix
	groupID := "group-" + suffix
	accountID := "account-" + suffix
	categoryID := "category-" + suffix
	now := time.Now().UTC().Truncate(time.Second)

	_, err = db.SQLDB().ExecContext(ctx,
		storage.Rebind(db.Dialect(), `INSERT INTO users (id, email, status, password_hash) VALUES (?, ?, ?, ?)`),
		userID, userID+"@example.test", "active", "hash")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.SQLDB().ExecContext(cleanupCtx, storage.Rebind(db.Dialect(), `DELETE FROM books WHERE id = ?`), bookID)
		_, _ = db.SQLDB().ExecContext(cleanupCtx, storage.Rebind(db.Dialect(), `DELETE FROM accounts WHERE id = ?`), accountID)
		_, _ = db.SQLDB().ExecContext(cleanupCtx, storage.Rebind(db.Dialect(), `DELETE FROM account_groups WHERE id = ?`), groupID)
		_, _ = db.SQLDB().ExecContext(cleanupCtx, storage.Rebind(db.Dialect(), `DELETE FROM users WHERE id = ?`), userID)
	})

	_, _, err = repo.CreateBook(ctx,
		ledger.Book{ID: bookID, OwnerUserID: userID, Name: "PG postings", ReportingCurrency: "USD", CreatedAt: now, UpdatedAt: now},
		ledger.BookMember{BookID: bookID, UserID: userID, Role: ledger.RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now},
		[]ledger.Category{{ID: categoryID, BookID: bookID, Name: "Food", Direction: ledger.CategoryDirectionExpense, SortOrder: 1, CreatedAt: now, UpdatedAt: now}})
	require.NoError(t, err)
	_, err = repo.CreateAccountGroup(ctx, ledger.AccountGroup{ID: groupID, UserID: userID, Name: "Cash", SortOrder: 1, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)
	account, err := repo.CreateAccount(ctx, ledger.Account{ID: accountID, UserID: userID, GroupID: groupID, Name: "Cash", Type: ledger.AccountTypeCash, Currency: "USD", SharedBookIDs: []string{bookID}, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)

	entryID, err := ledger.NewEntryID()
	require.NoError(t, err)
	_, err = repo.CreateEntry(ctx, ledger.Entry{
		ID: entryID, BookID: bookID, CreatorUserID: userID, Type: ledger.EntryTypeExpense,
		AccountID: account.ID, CategoryID: categoryID, AmountCents: 2500,
		TransactionCurrency: "USD", AccountCurrency: "USD", BookReportingCurrency: "USD",
		OccurredAt: now, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)

	var count int
	var debit, credit int64
	rows, err := db.SQLDB().QueryContext(ctx,
		storage.Rebind(db.Dialect(), `SELECT direction, reporting_cents FROM postings WHERE entry_id = ?`), entryID)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	for rows.Next() {
		var direction string
		var reportingCents int64
		require.NoError(t, rows.Scan(&direction, &reportingCents))
		count++
		if ledger.PostingDirection(direction) == ledger.PostingDebit {
			debit += reportingCents
		} else {
			credit += reportingCents
		}
	}
	require.NoError(t, rows.Err())
	require.Equal(t, 2, count)
	require.Equal(t, debit, credit)

	mismatches, err := repo.ReconcileBook(ctx, bookID)
	require.NoError(t, err)
	require.Equal(t, 0, mismatches)
}
