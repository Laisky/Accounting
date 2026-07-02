package ledger

import (
	"context"
	"path/filepath"
	"testing"

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
