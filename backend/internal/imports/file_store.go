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

// SnapshotStore persists import preview batches by writing the whole in-memory
// snapshot to an atomic JSON file.
type SnapshotStore struct {
	mu     sync.Mutex
	sink   persistence.SnapshotSink
	memory *MemoryStore
}

// NewFileStore receives a JSON path, loads existing batches, and returns a durable import store.
func NewFileStore(path string) (*SnapshotStore, error) {
	return newSnapshotStore(persistence.NewFileSink(path))
}

// newSnapshotStore loads the current snapshot from sink and returns a durable import store.
func newSnapshotStore(sink persistence.SnapshotSink) (*SnapshotStore, error) {
	var snapshot Snapshot
	if err := sink.Load(&snapshot); err != nil {
		return nil, errors.Wrap(err, "load imports store")
	}

	return &SnapshotStore{
		sink:   sink,
		memory: NewMemoryStoreFromSnapshot(snapshot),
	}, nil
}

// SaveBatch receives an import batch, stores it, and persists the snapshot.
func (s *SnapshotStore) SaveBatch(ctx context.Context, batch Batch) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, err := s.memory.SaveBatch(ctx, batch)
	if err != nil {
		return Batch{}, err
	}
	if err := s.sink.Save(s.memory.Snapshot()); err != nil {
		return Batch{}, errors.Wrap(err, "persist imports store")
	}

	return stored, nil
}

// SaveBatchIfAbsent atomically stores a new batch and persists the snapshot only
// when no batch already exists for the same owner/source/hash.
func (s *SnapshotStore) SaveBatchIfAbsent(ctx context.Context, batch Batch) (Batch, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, created, err := s.memory.SaveBatchIfAbsent(ctx, batch)
	if err != nil {
		return Batch{}, false, err
	}
	if !created {
		return stored, false, nil
	}
	if err := s.sink.Save(s.memory.Snapshot()); err != nil {
		return Batch{}, false, errors.Wrap(err, "persist imports store")
	}

	return stored, true, nil
}

// Batch receives owner and batch id values and returns the matching batch.
func (s *SnapshotStore) Batch(ctx context.Context, userID string, batchID string) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.Batch(ctx, userID, batchID)
}

// BatchByHash receives owner, source, and hash values and returns the matching batch.
func (s *SnapshotStore) BatchByHash(ctx context.Context, userID string, source string, sourceHash string) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.BatchByHash(ctx, userID, source, sourceHash)
}
