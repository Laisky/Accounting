package imports

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// TestMemoryStoreClaimForApplyExclusive verifies concurrent claims on one preview batch elect a single winner.
func TestMemoryStoreClaimForApplyExclusive(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.SaveBatch(context.Background(), previewBatch("u1", "b1"))
	require.NoError(t, err)

	assertExclusiveClaim(t, store, "u1", "b1")
}

// TestMemoryStoreApplyLifecycleTransitions verifies the preview→applying→applied CAS chain and compensation.
func TestMemoryStoreApplyLifecycleTransitions(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	_, err := store.SaveBatch(ctx, previewBatch("u1", "b1"))
	require.NoError(t, err)

	claimed, err := store.ClaimForApply(ctx, "u1", "b1")
	require.NoError(t, err)
	require.Equal(t, BatchStatusApplying, claimed.Status)

	// A second claim on an in-flight batch conflicts.
	_, err = store.ClaimForApply(ctx, "u1", "b1")
	require.ErrorIs(t, err, ErrConflict)

	// Reverting the claim restores preview and re-enables claiming.
	require.NoError(t, store.RevertToPreview(ctx, "u1", "b1"))
	reclaimed, err := store.ClaimForApply(ctx, "u1", "b1")
	require.NoError(t, err)
	require.Equal(t, BatchStatusApplying, reclaimed.Status)

	// Finalizing an applying batch records commit metadata.
	applied, err := store.FinalizeApplied(ctx, MarkAppliedRequest{Actor: Actor{UserID: "u1"}, BatchID: "b1", BookID: "book1", EntryIDs: []string{"e1", "e2"}})
	require.NoError(t, err)
	require.Equal(t, BatchStatusApplied, applied.Status)
	require.Equal(t, "book1", applied.AppliedBookID)

	// Finalizing again conflicts because the batch is no longer applying (idempotent replay is the service's job).
	_, err = store.FinalizeApplied(ctx, MarkAppliedRequest{Actor: Actor{UserID: "u1"}, BatchID: "b1", BookID: "book1", EntryIDs: []string{"e1"}})
	require.ErrorIs(t, err, ErrConflict)

	// Reverting a non-applying batch is a no-op.
	require.NoError(t, store.RevertToPreview(ctx, "u1", "b1"))
}

// TestSQLRepositoryClaimForApplySQLite exercises the conditional-UPDATE CAS on migrated sqlite.
func TestSQLRepositoryClaimForApplySQLite(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))

	runClaimContract(t, db)
}

// TestSQLRepositoryClaimForApplyPostgres runs the CAS contract against postgres when DATABASE_URL is set.
func TestSQLRepositoryClaimForApplyPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		t.Skip("DATABASE_URL not set; skipping postgres claim integration test")
	}

	ctx := context.Background()
	db, err := storage.Open(ctx, "postgres", databaseURL, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Migrate(ctx))

	runClaimContract(t, db)
}

// runClaimContract verifies exclusive claiming plus finalize/replay conflict on a migrated database.
func runClaimContract(t *testing.T, db *storage.DB) {
	t.Helper()

	ctx := context.Background()
	repo, err := NewSQLRepository(db)
	require.NoError(t, err)

	userID := "user_" + strings.ReplaceAll(t.Name(), "/", "_")
	bookID := "book_" + strings.ReplaceAll(t.Name(), "/", "_")
	cleanupImportBatches(t, db, userID)
	seedImportUser(t, db, userID)
	seedImportBook(t, db, bookID, userID)

	_, err = repo.SaveBatch(ctx, previewBatch(userID, "batch_claim"))
	require.NoError(t, err)

	assertExclusiveClaim(t, repo, userID, "batch_claim")

	// The exclusive winner left the batch applying; finalize it, then a second finalize conflicts.
	applied, err := repo.FinalizeApplied(ctx, MarkAppliedRequest{Actor: Actor{UserID: userID}, BatchID: "batch_claim", BookID: bookID, EntryIDs: []string{"e1", "e2"}})
	require.NoError(t, err)
	require.Equal(t, BatchStatusApplied, applied.Status)
	require.Equal(t, bookID, applied.AppliedBookID)
	require.Equal(t, []string{"e1", "e2"}, applied.AppliedEntryIDs)

	_, err = repo.FinalizeApplied(ctx, MarkAppliedRequest{Actor: Actor{UserID: userID}, BatchID: "batch_claim", BookID: bookID, EntryIDs: []string{"e1"}})
	require.ErrorIs(t, err, ErrConflict)
}

// previewBatch returns a minimal preview batch for CAS tests.
func previewBatch(userID string, batchID string) Batch {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	return Batch{
		ID:            batchID,
		UserID:        userID,
		Source:        "wacai",
		SourceHash:    "hash_" + batchID,
		ParserVersion: "wacai-v1",
		Status:        BatchStatusPreview,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// assertExclusiveClaim runs many concurrent claims and asserts exactly one wins and the rest fail.
func assertExclusiveClaim(t *testing.T, store Store, userID string, batchID string) {
	t.Helper()

	const workers = 16
	var wg sync.WaitGroup
	var successes int64
	var failures int64
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			if _, err := store.ClaimForApply(context.Background(), userID, batchID); err == nil {
				atomic.AddInt64(&successes, 1)
			} else {
				atomic.AddInt64(&failures, 1)
			}
		}()
	}
	wg.Wait()

	require.Equal(t, int64(1), successes, "exactly one concurrent claim must succeed")
	require.Equal(t, int64(workers-1), failures, "every other concurrent claim must fail")
}
