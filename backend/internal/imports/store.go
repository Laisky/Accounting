package imports

import (
	"context"
	"sync"

	"github.com/Laisky/errors/v2"
)

// Store defines import preview persistence operations required by the service layer.
type Store interface {
	SaveBatch(ctx context.Context, batch Batch) (Batch, error)
	Batch(ctx context.Context, userID string, batchID string) (Batch, error)
	BatchByHash(ctx context.Context, userID string, source string, sourceHash string) (Batch, error)
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

// NewMemoryStoreFromSnapshot receives durable state and returns an in-memory import Store implementation.
func NewMemoryStoreFromSnapshot(snapshot Snapshot) *MemoryStore {
	store := NewMemoryStore()
	for _, batch := range snapshot.Batches {
		batch = cloneBatch(batch)
		store.batches[batch.ID] = batch
		store.byHash[batchHashKey(batch.UserID, batch.Source, batch.SourceHash)] = batch.ID
	}

	return store
}

// Snapshot returns a detached durable representation of the store.
func (s *MemoryStore) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	batches := make([]Batch, 0, len(s.batches))
	for _, batch := range s.batches {
		batches = append(batches, cloneBatch(batch))
	}

	return Snapshot{Batches: batches}
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
