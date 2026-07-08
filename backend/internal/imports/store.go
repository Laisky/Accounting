package imports

import (
	"context"
	"sync"
	"time"

	"github.com/Laisky/errors/v2"
)

// Store defines import preview persistence operations required by the service layer.
type Store interface {
	SaveBatch(ctx context.Context, batch Batch) (Batch, error)
	SaveBatchIfAbsent(ctx context.Context, batch Batch) (Batch, bool, error)
	Batch(ctx context.Context, userID string, batchID string) (Batch, error)
	BatchByHash(ctx context.Context, userID string, source string, sourceHash string) (Batch, error)
	// ClaimForApply atomically transitions a batch from preview to applying, returning the
	// claimed batch. It returns ErrNotFound when the batch is absent or not owned by userID,
	// and ErrConflict when the batch is not in preview (already applying or applied). This CAS
	// is the single guard that serializes concurrent applies and closes the double-write window.
	ClaimForApply(ctx context.Context, userID string, batchID string) (Batch, error)
	// FinalizeApplied atomically transitions a claimed (applying) batch to applied, recording
	// the commit metadata. It returns ErrConflict when the batch is not in the applying state.
	FinalizeApplied(ctx context.Context, request MarkAppliedRequest) (Batch, error)
	// RevertToPreview atomically transitions an applying batch back to preview as the
	// compensating action for a failed apply. It is idempotent (a no-op when not applying).
	RevertToPreview(ctx context.Context, userID string, batchID string) error
}

// MemoryStore keeps import batches in process for the initial preview implementation.
type MemoryStore struct {
	mu      sync.RWMutex
	batches map[string]Batch
	byHash  map[string]string
}

// NewMemoryStore returns an empty in-memory import Store implementation.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		batches: map[string]Batch{},
		byHash:  map[string]string{},
	}
}

// SaveBatch receives an import batch and stores it by id and source hash.
func (s *MemoryStore) SaveBatch(_ context.Context, batch Batch) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	batch = cloneBatch(batch)
	s.batches[batch.ID] = batch
	s.byHash[batchHashKey(batch.UserID, batch.Source, batch.SourceHash)] = batch.ID

	return cloneBatch(batch), nil
}

// SaveBatchIfAbsent atomically stores a batch only when no batch already exists
// for the same owner/source/hash, returning the existing batch otherwise. This
// closes the check-then-save race where two concurrent identical uploads would
// each create a batch and orphan one under the hash index.
func (s *MemoryStore) SaveBatchIfAbsent(_ context.Context, batch Batch) (Batch, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := batchHashKey(batch.UserID, batch.Source, batch.SourceHash)
	if existingID, ok := s.byHash[key]; ok {
		return cloneBatch(s.batches[existingID]), false, nil
	}
	batch = cloneBatch(batch)
	s.batches[batch.ID] = batch
	s.byHash[key] = batch.ID

	return cloneBatch(batch), true, nil
}

// Batch receives owner and batch id values and returns the matching batch.
func (s *MemoryStore) Batch(_ context.Context, userID string, batchID string) (Batch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	batch, ok := s.batches[batchID]
	if !ok || batch.UserID != userID {
		return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
	}

	return cloneBatch(batch), nil
}

// BatchByHash receives owner, source, and hash values and returns the matching batch.
func (s *MemoryStore) BatchByHash(_ context.Context, userID string, source string, sourceHash string) (Batch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byHash[batchHashKey(userID, source, sourceHash)]
	if !ok {
		return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
	}

	return cloneBatch(s.batches[id]), nil
}

// ClaimForApply atomically transitions a batch from preview to applying under the store mutex.
func (s *MemoryStore) ClaimForApply(_ context.Context, userID string, batchID string) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	batch, ok := s.batches[batchID]
	if !ok || batch.UserID != userID {
		return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
	}
	if batch.Status != BatchStatusPreview {
		return Batch{}, errors.Wrapf(ErrConflict, "import batch is %s", batch.Status)
	}
	batch.Status = BatchStatusApplying
	batch.UpdatedAt = time.Now().UTC()
	s.batches[batchID] = cloneBatch(batch)

	return cloneBatch(batch), nil
}

// FinalizeApplied atomically transitions a claimed batch to applied, recording commit metadata.
func (s *MemoryStore) FinalizeApplied(_ context.Context, request MarkAppliedRequest) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	batch, ok := s.batches[request.BatchID]
	if !ok || batch.UserID != request.Actor.UserID {
		return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
	}
	if batch.Status != BatchStatusApplying {
		return Batch{}, errors.Wrapf(ErrConflict, "import batch is %s", batch.Status)
	}
	now := time.Now().UTC()
	batch.Status = BatchStatusApplied
	batch.AppliedBookID = request.BookID
	batch.AppliedEntryIDs = append([]string(nil), request.EntryIDs...)
	batch.AppliedSkippedRows = cloneAppliedSkippedRows(request.SkippedRows)
	batch.AppliedAt = &now
	batch.UpdatedAt = now
	s.batches[request.BatchID] = cloneBatch(batch)

	return cloneBatch(batch), nil
}

// RevertToPreview atomically returns an applying batch to preview; it is a no-op otherwise.
func (s *MemoryStore) RevertToPreview(_ context.Context, userID string, batchID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	batch, ok := s.batches[batchID]
	if !ok || batch.UserID != userID {
		return errors.Wrap(ErrNotFound, "import batch not found")
	}
	if batch.Status != BatchStatusApplying {
		return nil
	}
	batch.Status = BatchStatusPreview
	batch.UpdatedAt = time.Now().UTC()
	s.batches[batchID] = cloneBatch(batch)

	return nil
}

// batchHashKey receives batch identity fields and returns a stable lookup key.
func batchHashKey(userID string, source string, sourceHash string) string {
	return userID + "\x00" + source + "\x00" + sourceHash
}

// cloneBatch receives an import batch and returns a detached copy.
func cloneBatch(batch Batch) Batch {
	batch.Rows = cloneRows(batch.Rows)
	batch.DetectedSchema.Columns = cloneStringMap(batch.DetectedSchema.Columns)
	batch.DetectedSchema.Missing = append([]string(nil), batch.DetectedSchema.Missing...)
	batch.Detected = cloneDetectedValues(batch.Detected)
	batch.AppliedEntryIDs = append([]string(nil), batch.AppliedEntryIDs...)
	batch.AppliedSkippedRows = cloneAppliedSkippedRows(batch.AppliedSkippedRows)
	if batch.AppliedAt != nil {
		appliedAt := batch.AppliedAt.UTC()
		batch.AppliedAt = &appliedAt
	}
	return batch
}

// cloneRows receives preview rows and returns detached copies.
func cloneRows(rows []PreviewRow) []PreviewRow {
	cloned := make([]PreviewRow, 0, len(rows))
	for _, row := range rows {
		row.Raw = cloneStringMap(row.Raw)
		row.Tags = append([]string(nil), row.Tags...)
		row.Warnings = append([]string(nil), row.Warnings...)
		row.Errors = append([]string(nil), row.Errors...)
		cloned = append(cloned, row)
	}

	return cloned
}

// cloneStringMap receives a string map and returns a detached copy.
func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}

	return cloned
}

// cloneDetectedValues receives detected values and returns detached slices.
func cloneDetectedValues(values DetectedValues) DetectedValues {
	values.Books = append([]string(nil), values.Books...)
	values.Accounts = append([]string(nil), values.Accounts...)
	values.Categories = append([]string(nil), values.Categories...)
	values.Currencies = append([]string(nil), values.Currencies...)
	values.Members = append([]string(nil), values.Members...)
	values.Merchants = append([]string(nil), values.Merchants...)
	values.Tags = append([]string(nil), values.Tags...)
	return values
}

// cloneAppliedSkippedRows receives skipped row metadata and returns detached copies.
func cloneAppliedSkippedRows(rows []AppliedSkippedRow) []AppliedSkippedRow {
	return append([]AppliedSkippedRow(nil), rows...)
}
