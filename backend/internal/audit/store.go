package audit

import (
	"context"
	"sync"

	"github.com/Laisky/errors/v2"
)

// Store defines audit persistence operations required by the service layer.
type Store interface {
	SaveEvent(ctx context.Context, event Event) (Event, error)
	EventsByActor(ctx context.Context, actorID string) ([]Event, error)
	AllEvents(ctx context.Context) ([]Event, error)
	Tail(ctx context.Context) (Event, error)
}

// MemoryStore keeps audit events in process for local development.
type MemoryStore struct {
	mu     sync.RWMutex
	events []Event
}

// NewMemoryStore returns an empty in-memory audit Store implementation.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{events: []Event{}}
}

// NewMemoryStoreFromSnapshot receives durable state and returns an in-memory audit Store implementation.
func NewMemoryStoreFromSnapshot(snapshot Snapshot) *MemoryStore {
	events := make([]Event, 0, len(snapshot.Events))
	for _, event := range snapshot.Events {
		events = append(events, cloneEvent(event))
	}

	return &MemoryStore{events: events}
}

// Snapshot returns a detached durable representation of the store.
func (s *MemoryStore) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		events = append(events, cloneEvent(event))
	}

	return Snapshot{Events: events}
}

// SaveEvent receives an event and stores a detached copy.
func (s *MemoryStore) SaveEvent(_ context.Context, event Event) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := cloneEvent(event)
	s.events = append(s.events, stored)
	return cloneEvent(stored), nil
}

// EventsByActor receives an actor id and returns matching events newest first.
func (s *MemoryStore) EventsByActor(_ context.Context, actorID string) ([]Event, error) {
	if actorID == "" {
		return nil, errors.Wrap(ErrInvalidInput, "actor id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]Event, 0)
	for index := len(s.events) - 1; index >= 0; index-- {
		event := s.events[index]
		if event.ActorID == actorID {
			events = append(events, cloneEvent(event))
		}
	}

	return events, nil
}

// AllEvents returns all audit events newest first.
func (s *MemoryStore) AllEvents(_ context.Context) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]Event, 0, len(s.events))
	for index := len(s.events) - 1; index >= 0; index-- {
		events = append(events, cloneEvent(s.events[index]))
	}

	return events, nil
}

// Tail returns the newest stored event in append order.
func (s *MemoryStore) Tail(_ context.Context) (Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.events) == 0 {
		return Event{}, errors.WithStack(ErrNotFound)
	}

	return cloneEvent(s.events[len(s.events)-1]), nil
}

// cloneEvent receives an event and returns a detached copy.
func cloneEvent(event Event) Event {
	if event.Metadata != nil {
		metadata := make(map[string]string, len(event.Metadata))
		for key, value := range event.Metadata {
			metadata[key] = value
		}
		event.Metadata = metadata
	}

	return event
}
