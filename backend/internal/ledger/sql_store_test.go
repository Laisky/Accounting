package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// TestSQLStorePersistsBookAcrossStoreInstances verifies SQL writes are durable
// immediately and do not depend on an in-memory snapshot flush.
func TestSQLStorePersistsBookAcrossStoreInstances(t *testing.T) {
	db, dialect, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, persistence.DialectSQLite, dialect)

	store, err := NewSQLiteStore(db, SeedData{})
	require.NoError(t, err)

	now := time.Now().UTC()
	book := Book{
		ID:                "book_sql",
		OwnerUserID:       "user_1",
		Name:              "SQL Book",
		ReportingCurrency: "USD",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	member := BookMember{
		BookID:    book.ID,
		UserID:    book.OwnerUserID,
		Role:      RoleOwner,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, _, err = store.CreateBook(context.Background(), book, member)
	require.NoError(t, err)

	reopened, err := NewSQLiteStore(db, SeedData{})
	require.NoError(t, err)
	loaded, err := reopened.Book(context.Background(), book.ID)
	require.NoError(t, err)
	require.Equal(t, book.ID, loaded.ID)

	members, err := reopened.BookMemberships(context.Background(), book.OwnerUserID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, book.ID, members[0].BookID)
}

// TestSQLStorePersistsEntryDelete verifies deletes are committed to SQL immediately.
func TestSQLStorePersistsEntryDelete(t *testing.T) {
	db, _, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	now := time.Now().UTC()
	store, err := NewSQLiteStore(db, SeedData{
		Books: []Book{{
			ID:                "book_entries",
			OwnerUserID:       "user_1",
			Name:              "Entries",
			ReportingCurrency: "USD",
			CreatedAt:         now,
			UpdatedAt:         now,
		}},
		Members: []BookMember{{
			BookID:    "book_entries",
			UserID:    "user_1",
			Role:      RoleOwner,
			CreatedAt: now,
			UpdatedAt: now,
		}},
	})
	require.NoError(t, err)

	entry := Entry{
		ID:                    "entry_sql",
		BookID:                "book_entries",
		CreatorUserID:         "user_1",
		Type:                  EntryTypeExpense,
		AmountCents:           1200,
		TransactionCurrency:   "USD",
		AccountCurrency:       "USD",
		BookReportingCurrency: "USD",
		OccurredAt:            now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	_, err = store.CreateEntry(context.Background(), entry)
	require.NoError(t, err)
	require.NoError(t, store.DeleteEntry(context.Background(), entry.BookID, entry.ID))

	reopened, err := NewSQLiteStore(db, SeedData{})
	require.NoError(t, err)
	_, err = reopened.Entry(context.Background(), entry.BookID, entry.ID)
	require.ErrorIs(t, err, ErrNotFound)
}
