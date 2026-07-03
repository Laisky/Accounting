package ledger

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestFileStorePersistsLedgerMutations verifies ledger mutations survive reopening the JSON store.
func TestFileStorePersistsLedgerMutations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	store, err := NewFileStore(path, SeedData{})
	require.NoError(t, err)
	service := NewServiceWithStore(store)

	book, err := service.CreateBook(context.Background(), CreateBookRequest{
		Actor:             Actor{UserID: "user-1"},
		Name:              "Household",
		ReportingCurrency: "USD",
	})
	require.NoError(t, err)

	reopened, err := NewFileStore(path, SeedData{})
	require.NoError(t, err)
	books, err := NewServiceWithStore(reopened).ListBooks(context.Background(), ListBooksRequest{
		Actor: Actor{UserID: "user-1"},
	})
	require.NoError(t, err)
	require.Len(t, books.Items, 1)
	require.Equal(t, book.ID, books.Items[0].ID)
}

// TestFileStorePersistsEntryUUID verifies entry UUIDs survive reopening the JSON store.
func TestFileStorePersistsEntryUUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	store, err := NewFileStore(path, testSeedData())
	require.NoError(t, err)
	service := NewServiceWithStore(store)

	entry, err := service.CreateEntry(context.Background(), CreateEntryRequest{
		Actor:               Actor{UserID: "member"},
		BookID:              "book",
		Type:                EntryTypeExpense,
		AccountID:           "account-member",
		AmountCents:         1200,
		TransactionCurrency: "USD",
		OccurredAt:          time.Date(2026, 7, 1, 20, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	parsedID, err := uuid.Parse(entry.ID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsedID.Version())

	reopened, err := NewFileStore(path, SeedData{})
	require.NoError(t, err)
	loaded, err := reopened.Entry(context.Background(), entry.BookID, entry.ID)
	require.NoError(t, err)
	require.Equal(t, entry.ID, loaded.ID)
}
