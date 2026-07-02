package imports

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// TestSQLStoreSaveBatchIfAbsentPersists verifies hash-deduped batches are
// written to SQL and reused by fresh store instances.
func TestSQLStoreSaveBatchIfAbsentPersists(t *testing.T) {
	db, _, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	now := time.Now().UTC()
	batch := Batch{
		ID:            "batch_sql",
		UserID:        "user_1",
		Source:        "wacai",
		Filename:      "records.csv",
		ContentType:   "text/csv",
		SourceHash:    "hash_1",
		ParserVersion: "test",
		Status:        BatchStatusPreview,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	stored, created, err := store.SaveBatchIfAbsent(context.Background(), batch)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, batch.ID, stored.ID)

	reopened := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	stored, created, err = reopened.SaveBatchIfAbsent(context.Background(), Batch{
		ID:         "other_batch",
		UserID:     batch.UserID,
		Source:     batch.Source,
		SourceHash: batch.SourceHash,
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, batch.ID, stored.ID)
}
