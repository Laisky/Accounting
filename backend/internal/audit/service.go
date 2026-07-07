package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
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
		ActorEmail: strings.ToLower(strings.TrimSpace(request.ActorEmail)),
		Action:     request.Action,
		TargetType: strings.TrimSpace(request.TargetType),
		TargetID:   strings.TrimSpace(request.TargetID),
		Metadata:   sanitizeMetadata(request.Metadata),
		CreatedAt:  s.clock().UTC(),
	}
	if err := s.fillChain(ctx, &event); err != nil {
		return Event{}, err
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

	return ListResult{
		Items:    paginateEvents(events, page, pageSize),
		Page:     page,
		PageSize: pageSize,
		Total:    len(events),
	}, nil
}

// ListAll receives a paginated request and returns all audit events.
func (s *Service) ListAll(ctx context.Context, request ListRequest) (ListResult, error) {
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

	events, err := s.store.AllEvents(ctx)
	if err != nil {
		return ListResult{}, errors.Wrap(err, "load audit events")
	}

	return ListResult{
		Items:    paginateEvents(events, page, pageSize),
		Page:     page,
		PageSize: pageSize,
		Total:    len(events),
	}, nil
}

// VerifyChain receives audit events and verifies their hash-chain continuity.
func VerifyChain(events []Event) error {
	if len(events) == 0 {
		return nil
	}
	ordered := make([]Event, 0, len(events))
	for _, event := range events {
		ordered = append(ordered, cloneEvent(event))
	}
	slices.SortFunc(ordered, func(left Event, right Event) int {
		if left.Seq < right.Seq {
			return -1
		}
		if left.Seq > right.Seq {
			return 1
		}
		return 0
	})

	prevHash := ""
	for index, event := range ordered {
		expectedSeq := int64(index + 1)
		if event.Seq != expectedSeq {
			return errors.Errorf("audit chain seq mismatch at %s", event.ID)
		}
		if event.PrevHash != prevHash {
			return errors.Errorf("audit chain previous hash mismatch at %s", event.ID)
		}
		hash, err := eventHash(event)
		if err != nil {
			return err
		}
		if event.Hash != hash {
			return errors.Errorf("audit chain hash mismatch at %s", event.ID)
		}
		prevHash = event.Hash
	}

	return nil
}

// SubjectHash receives an auth subject such as an email address and returns a stable audit-safe hash.
func SubjectHash(subject string) string {
	subject = strings.ToLower(strings.TrimSpace(subject))
	if subject == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(subject))
	return hex.EncodeToString(sum[:])
}

func (s *Service) fillChain(ctx context.Context, event *Event) error {
	tail, err := s.store.Tail(ctx)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return errors.Wrap(err, "load audit tail")
	}
	if err == nil {
		event.Seq = tail.Seq + 1
		event.PrevHash = tail.Hash
	} else {
		event.Seq = 1
	}
	hash, err := eventHash(*event)
	if err != nil {
		return err
	}
	event.Hash = hash
	return nil
}

func paginateEvents(events []Event, page int, pageSize int) []Event {
	start := (page - 1) * pageSize
	// Guard against out-of-range pages and int overflow: a very large page makes
	// (page-1)*pageSize wrap negative, which would produce a negative slice bound
	// and panic. Treat any such page as past the end (empty result).
	if start < 0 || start > len(events) {
		start = len(events)
	}
	end := start + pageSize
	if end > len(events) {
		end = len(events)
	}

	return events[start:end]
}

func eventHash(event Event) (string, error) {
	payload := struct {
		Seq        int64             `json:"seq"`
		PrevHash   string            `json:"prevHash"`
		ID         string            `json:"id"`
		ActorID    string            `json:"actorId"`
		ActorEmail string            `json:"actorEmail"`
		Action     Action            `json:"action"`
		TargetType string            `json:"targetType"`
		TargetID   string            `json:"targetId"`
		Metadata   map[string]string `json:"metadata,omitempty"`
		CreatedAt  string            `json:"createdAt"`
	}{
		Seq:        event.Seq,
		PrevHash:   event.PrevHash,
		ID:         event.ID,
		ActorID:    event.ActorID,
		ActorEmail: event.ActorEmail,
		Action:     event.Action,
		TargetType: event.TargetType,
		TargetID:   event.TargetID,
		Metadata:   event.Metadata,
		CreatedAt:  event.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", errors.Wrap(err, "encode audit chain payload")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
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
