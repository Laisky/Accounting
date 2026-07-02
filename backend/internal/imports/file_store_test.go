package imports

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFileStorePersistsBatches verifies import batches survive reopening the JSON store.
func TestFileStorePersistsBatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imports.json")
	store, err := NewFileStore(path)
	require.NoError(t, err)

	_, err = store.SaveBatch(context.Background(), Batch{
		ID:            "batch-1",
		UserID:        "user-1",
		Source:        "wacai",
		Filename:      "wacai.csv",
		SourceHash:    "hash-1",
		ParserVersion: "test",
		Status:        BatchStatusPreview,
		CreatedAt:     time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	reopened, err := NewFileStore(path)
	require.NoError(t, err)
	batch, err := reopened.BatchByHash(context.Background(), "user-1", "wacai", "hash-1")
	require.NoError(t, err)
	require.Equal(t, "batch-1", batch.ID)
}
