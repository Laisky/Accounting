package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

const auditEventsNS = "audit.events"

// SQLStore persists audit events directly in SQL rows.
type SQLStore struct {
	records *persistence.RecordStore
}

// NewSQLStore receives a record store and returns a direct SQL audit Store implementation.
func NewSQLStore(records *persistence.RecordStore) *SQLStore {
	return &SQLStore{records: records}
}

// NewPostgresStore receives a database handle and returns a direct SQL audit store.
func NewPostgresStore(db *sql.DB) (*SQLStore, error) {
	return NewSQLStore(persistence.NewRecordStore(db, persistence.DialectPostgres)), nil
}

// NewSQLiteStore receives a database handle and returns a direct SQL audit store.
func NewSQLiteStore(db *sql.DB) (*SQLStore, error) {
	return NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite)), nil
}

// SaveEvent receives an event and stores a detached copy.
func (s *SQLStore) SaveEvent(ctx context.Context, event Event) (Event, error) {
	stored := cloneEvent(event)
	data, err := json.Marshal(stored)
	if err != nil {
		return Event{}, errors.Wrap(err, "encode audit event")
	}
	if err := s.records.Insert(ctx, persistence.Record{
		Namespace: auditEventsNS,
		Key:       stored.ID,
		OwnerKey:  stored.ActorID,
		Data:      data,
	}); err != nil {
		return Event{}, errors.Wrap(err, "insert audit event")
	}
	return cloneEvent(stored), nil
}

// EventsByActor receives an actor id and returns matching events newest first.
func (s *SQLStore) EventsByActor(ctx context.Context, actorID string) ([]Event, error) {
	if actorID == "" {
		return nil, errors.Wrap(ErrInvalidInput, "actor id is required")
	}
	var events []Event
	if err := s.records.List(ctx, auditEventsNS, nil, &actorID, &events); err != nil {
		return nil, errors.Wrap(err, "list audit events")
	}
	return newestFirst(events), nil
}

// AllEvents returns all audit events newest first.
func (s *SQLStore) AllEvents(ctx context.Context) ([]Event, error) {
	var events []Event
	if err := s.records.List(ctx, auditEventsNS, nil, nil, &events); err != nil {
		return nil, errors.Wrap(err, "list all audit events")
	}
	return newestFirst(events), nil
}

// Tail returns the highest-sequence audit event.
func (s *SQLStore) Tail(ctx context.Context) (Event, error) {
	events, err := s.AllEvents(ctx)
	if err != nil {
		return Event{}, err
	}
	if len(events) == 0 {
		return Event{}, errors.WithStack(ErrNotFound)
	}
	return cloneEvent(events[0]), nil
}

func newestFirst(events []Event) []Event {
	slices.SortFunc(events, func(left Event, right Event) int {
		if left.Seq > right.Seq {
			return -1
		}
		if left.Seq < right.Seq {
			return 1
		}
		return right.CreatedAt.Compare(left.CreatedAt)
	})
	cloned := make([]Event, 0, len(events))
	for _, event := range events {
		cloned = append(cloned, cloneEvent(event))
	}
	return cloned
}
