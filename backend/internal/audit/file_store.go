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

// FileStore persists audit events to an atomic JSON snapshot file.
type FileStore struct {
	mu     sync.Mutex
	path   string
	memory *MemoryStore
}

// NewFileStore receives a JSON path, loads existing events, and returns a durable audit store.
func NewFileStore(path string) (*FileStore, error) {
	var snapshot Snapshot
	if err := persistence.LoadJSON(path, &snapshot); err != nil {
		return nil, errors.Wrap(err, "load audit file store")
	}

	return &FileStore{
		path:   path,
		memory: NewMemoryStoreFromSnapshot(snapshot),
	}, nil
}

// SaveEvent receives an event, stores it, and persists the snapshot.
func (s *FileStore) SaveEvent(ctx context.Context, event Event) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, err := s.memory.SaveEvent(ctx, event)
	if err != nil {
		return Event{}, err
	}
	if err := persistence.SaveJSONAtomic(s.path, s.memory.Snapshot()); err != nil {
		return Event{}, errors.Wrap(err, "persist audit file store")
	}

	return stored, nil
}

// EventsByActor receives an actor id and returns matching events newest first.
func (s *FileStore) EventsByActor(ctx context.Context, actorID string) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.memory.EventsByActor(ctx, actorID)
}
