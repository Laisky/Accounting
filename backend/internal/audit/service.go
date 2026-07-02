package audit

import (
	"context"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
)

const (
	defaultPage     = 1
	defaultPageSize = 50
	maxPageSize     = 100
)

// Clock returns the current time for testable UTC audit timestamps.
type Clock func() time.Time

// Service owns audit event use cases over an audit Store.
type Service struct {
	clock Clock
	store Store
}

// NewService receives a store and returns an audit Service.
func NewService(store Store) *Service {
	if store == nil {
		store = NewMemoryStore()
	}

	return &Service{
		clock: func() time.Time { return time.Now().UTC() },
		store: store,
	}
}

// WithClock receives a Clock and returns the service after installing it for tests.
func (s *Service) WithClock(clock Clock) *Service {
	if clock != nil {
		s.clock = clock
	}

	return s
}

// Record receives audit event input, sanitizes it, stores it, and returns the event.
func (s *Service) Record(ctx context.Context, request RecordRequest) (Event, error) {
	if request.Action == "" {
		return Event{}, errors.Wrap(ErrInvalidInput, "action is required")
	}
	if strings.TrimSpace(request.TargetType) == "" {
		return Event{}, errors.Wrap(ErrInvalidInput, "target type is required")
	}

	event := Event{
		ID:         uuid.NewString(),
		ActorID:    strings.TrimSpace(request.ActorID),
		ActorEmail: strings.TrimSpace(request.ActorEmail),
		Action:     request.Action,
		TargetType: strings.TrimSpace(request.TargetType),
		TargetID:   strings.TrimSpace(request.TargetID),
		Metadata:   sanitizeMetadata(request.Metadata),
		CreatedAt:  s.clock().UTC(),
	}
	stored, err := s.store.SaveEvent(ctx, event)
	if err != nil {
		return Event{}, errors.Wrap(err, "save audit event")
	}

	return stored, nil
}

// List receives an actor-scoped request and returns paginated audit events.
func (s *Service) List(ctx context.Context, request ListRequest) (ListResult, error) {
	if strings.TrimSpace(request.ActorID) == "" {
		return ListResult{}, errors.Wrap(ErrInvalidInput, "actor id is required")
	}

	page := request.Page
	if page == 0 {
		page = defaultPage
	}
	pageSize := request.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if page < 1 || pageSize < 1 || pageSize > maxPageSize {
		return ListResult{}, errors.Wrap(ErrInvalidInput, "pagination is invalid")
	}

	events, err := s.store.EventsByActor(ctx, request.ActorID)
	if err != nil {
		return ListResult{}, errors.Wrap(err, "load audit events")
	}

	start := (page - 1) * pageSize
	if start > len(events) {
		start = len(events)
	}
	end := start + pageSize
	if end > len(events) {
		end = len(events)
	}

	return ListResult{
		Items:    events[start:end],
		Page:     page,
		PageSize: pageSize,
		Total:    len(events),
	}, nil
}

// sanitizeMetadata receives arbitrary metadata and returns trimmed non-secret values.
func sanitizeMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}

	safe := make(map[string]string, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" || isSensitiveMetadataKey(key) {
			continue
		}
		if len(value) > 256 {
			value = value[:256]
		}
		safe[key] = value
	}
	if len(safe) == 0 {
		return nil
	}

	return safe
}

// isSensitiveMetadataKey receives a metadata key and reports whether it must be omitted.
func isSensitiveMetadataKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "password") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "code")
}
