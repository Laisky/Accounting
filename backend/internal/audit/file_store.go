package audit

import (
	"context"
	"sync"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// Snapshot contains the durable audit store state.
type Snapshot struct {
	Events []Event `json:"events"`
}

// SnapshotStore persists audit events by writing the whole in-memory snapshot to
// an atomic JSON file after each write.
type SnapshotStore struct {
	mu     sync.Mutex
	sink   persistence.SnapshotSink
	memory *MemoryStore
}

// NewFileStore receives a JSON path, loads existing events, and returns a durable audit store.
func NewFileStore(path string) (*SnapshotStore, error) {
	return newSnapshotStore(persistence.NewFileSink(path))
}

// newSnapshotStore loads the current snapshot from sink and returns a durable audit store.
func newSnapshotStore(sink persistence.SnapshotSink) (*SnapshotStore, error) {
	var snapshot Snapshot
	if err := sink.Load(&snapshot); err != nil {
		return nil, errors.Wrap(err, "load audit store")
	}

	return &SnapshotStore{
		sink:   sink,
		memory: NewMemoryStoreFromSnapshot(snapshot),
	}, nil
}

// SaveEvent receives an event, stores it, and persists the snapshot.
func (s *SnapshotStore) SaveEvent(ctx context.Context, event Event) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, err := s.memory.SaveEvent(ctx, event)
	if err != nil {
		return Event{}, err
	}
	if err := s.sink.Save(s.memory.Snapshot()); err != nil {
		return Event{}, errors.Wrap(err, "persist audit store")
	}

	return stored, nil
}

// EventsByActor receives an actor id and returns matching events newest first.
func (s *SnapshotStore) EventsByActor(ctx context.Context, actorID string) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.EventsByActor(ctx, actorID)
}
