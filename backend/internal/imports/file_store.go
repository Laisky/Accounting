package imports

import (
	"context"
	"sync"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// Snapshot contains the durable import preview store state.
type Snapshot struct {
	Batches []Batch `json:"batches"`
}

// FileStore persists import preview batches to an atomic JSON snapshot file.
type FileStore struct {
	mu     sync.Mutex
	path   string
	memory *MemoryStore
}

// NewFileStore receives a JSON path, loads existing batches, and returns a durable import store.
func NewFileStore(path string) (*FileStore, error) {
	var snapshot Snapshot
	if err := persistence.LoadJSON(path, &snapshot); err != nil {
		return nil, errors.Wrap(err, "load imports file store")
	}

	return &FileStore{
		path:   path,
		memory: NewMemoryStoreFromSnapshot(snapshot),
	}, nil
}

// SaveBatch receives an import batch, stores it, and persists the snapshot.
func (s *FileStore) SaveBatch(ctx context.Context, batch Batch) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, err := s.memory.SaveBatch(ctx, batch)
	if err != nil {
		return Batch{}, err
	}
	if err := persistence.SaveJSONAtomic(s.path, s.memory.Snapshot()); err != nil {
		return Batch{}, errors.Wrap(err, "persist imports file store")
	}

	return stored, nil
}

// Batch receives owner and batch id values and returns the matching batch.
func (s *FileStore) Batch(ctx context.Context, userID string, batchID string) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.Batch(ctx, userID, batchID)
}

// BatchByHash receives owner, source, and hash values and returns the matching batch.
func (s *FileStore) BatchByHash(ctx context.Context, userID string, source string, sourceHash string) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.BatchByHash(ctx, userID, source, sourceHash)
}
