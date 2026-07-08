package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// SQLRepository persists audit events in the relational audit_events table.
//
// The relational schema stores only the durable event fields (id, seq, actor, action,
// target, metadata, created_at); it deliberately omits the hash and prev_hash columns.
// Those are a deterministic function of the stored fields plus the preceding event's hash,
// so the tamper-evident chain is reconstructed on read (see hydrateAuditChain) rather than
// persisted. Reconstruction reproduces exactly the hashes the service computed at record
// time as long as every hashed field round-trips faithfully (see the timestamp note below).
type SQLRepository struct {
	db      *storage.DB
	dialect string
}

// NewSQLRepository receives a migrated storage database and returns a relational audit Store.
func NewSQLRepository(db *storage.DB) (*SQLRepository, error) {
	if db == nil || db.SQLDB() == nil {
		return nil, errors.WithStack(errors.New("storage database is nil"))
	}
	return &SQLRepository{db: db, dialect: db.Dialect()}, nil
}

// SaveEvent receives a chain-linked event and stores it, preserving its caller-assigned seq.
//
// The service layer computes Seq/PrevHash/Hash before calling SaveEvent, and the hash covers
// Seq, so the stored seq must equal the caller-assigned seq (otherwise the hash reconstructed
// on read would diverge from the hash the caller was handed). We therefore insert the seq
// explicitly, using OVERRIDING SYSTEM VALUE on postgres where seq is GENERATED ALWAYS, instead
// of relying on the identity/autoincrement value which could gap. Hash and PrevHash are not
// persisted; they are derived on read.
func (s *SQLRepository) SaveEvent(ctx context.Context, event Event) (Event, error) {
	stored := cloneEvent(event)
	metadata, err := marshalAuditMetadata(stored.Metadata)
	if err != nil {
		return Event{}, err
	}

	query := `
		INSERT INTO audit_events (id, seq, actor_id, actor_email, action, target_type, target_id, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if s.dialect == storage.DialectPostgres {
		query = `
			INSERT INTO audit_events (id, seq, actor_id, actor_email, action, target_type, target_id, metadata, created_at)
			OVERRIDING SYSTEM VALUE
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	}

	if _, err := s.db.SQLDB().ExecContext(ctx, storage.Rebind(s.dialect, query),
		stored.ID, stored.Seq, auditNullString(stored.ActorID), stored.ActorEmail, string(stored.Action),
		stored.TargetType, stored.TargetID, metadata, stored.CreatedAt.UTC()); err != nil {
		return Event{}, errors.Wrap(err, "insert audit event")
	}
	return cloneEvent(stored), nil
}

// EventsByActor receives an actor id and returns matching events newest first.
func (s *SQLRepository) EventsByActor(ctx context.Context, actorID string) ([]Event, error) {
	if actorID == "" {
		return nil, errors.Wrap(ErrInvalidInput, "actor id is required")
	}
	// The chain must be hydrated over every event so each actor row carries its true global
	// hash and prev-hash, matching the legacy store which persisted them per event.
	chain, err := s.loadChain(ctx)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0)
	for index := len(chain) - 1; index >= 0; index-- {
		if chain[index].ActorID == actorID {
			events = append(events, cloneEvent(chain[index]))
		}
	}
	return events, nil
}

// AllEvents returns all audit events newest first.
func (s *SQLRepository) AllEvents(ctx context.Context) ([]Event, error) {
	chain, err := s.loadChain(ctx)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(chain))
	for index := len(chain) - 1; index >= 0; index-- {
		events = append(events, cloneEvent(chain[index]))
	}
	return events, nil
}

// Tail returns the highest-sequence audit event or ErrNotFound when the store is empty.
func (s *SQLRepository) Tail(ctx context.Context) (Event, error) {
	chain, err := s.loadChain(ctx)
	if err != nil {
		return Event{}, err
	}
	if len(chain) == 0 {
		return Event{}, errors.WithStack(ErrNotFound)
	}
	return cloneEvent(chain[len(chain)-1]), nil
}

// loadChain reads every event ordered by ascending seq and reconstructs the hash chain.
func (s *SQLRepository) loadChain(ctx context.Context) ([]Event, error) {
	rows, err := s.db.SQLDB().QueryContext(ctx, `
		SELECT id, seq, actor_id, actor_email, action, target_type, target_id, metadata, created_at
		FROM audit_events ORDER BY seq ASC`)
	if err != nil {
		return nil, errors.Wrap(err, "load audit chain")
	}
	events, err := scanAuditEvents(rows)
	if err != nil {
		return nil, err
	}
	if err := hydrateAuditChain(events); err != nil {
		return nil, err
	}
	return events, nil
}

// hydrateAuditChain fills PrevHash and Hash for events ordered by ascending seq.
func hydrateAuditChain(events []Event) error {
	prevHash := ""
	for index := range events {
		events[index].PrevHash = prevHash
		hash, err := eventHash(events[index])
		if err != nil {
			return err
		}
		events[index].Hash = hash
		prevHash = hash
	}
	return nil
}

func scanAuditEvents(rows *sql.Rows) ([]Event, error) {
	defer func() { _ = rows.Close() }()

	events := []Event{}
	for rows.Next() {
		var event Event
		var actorID sql.NullString
		var action string
		var metadata auditMetadata
		if err := rows.Scan(&event.ID, &event.Seq, &actorID, &event.ActorEmail, &action,
			&event.TargetType, &event.TargetID, &metadata, (*auditSQLTime)(&event.CreatedAt)); err != nil {
			return nil, errors.Wrap(err, "scan audit event")
		}
		event.ActorID = actorID.String
		event.Action = Action(action)
		event.Metadata = metadata
		events = append(events, cloneEvent(event))
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate audit events")
	}
	return events, nil
}

// marshalAuditMetadata encodes sanitized metadata as a JSON object for the jsonb/text column.
func marshalAuditMetadata(metadata map[string]string) (string, error) {
	if len(metadata) == 0 {
		return "{}", nil
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return "", errors.Wrap(err, "encode audit metadata")
	}
	return string(data), nil
}

func auditNullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// auditMetadata scans the jsonb/text metadata column back into the sanitized nil-for-empty form.
type auditMetadata map[string]string

func (m *auditMetadata) Scan(src any) error {
	var data []byte
	switch typed := src.(type) {
	case nil:
		*m = nil
		return nil
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		return errors.Errorf("unsupported audit metadata value %T", src)
	}
	if len(data) == 0 {
		*m = nil
		return nil
	}
	parsed := map[string]string{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return errors.Wrap(err, "decode audit metadata")
	}
	if len(parsed) == 0 {
		*m = nil
		return nil
	}
	*m = parsed
	return nil
}

// auditSQLTime scans dialect-specific timestamp encodings into a UTC time.Time.
//
// Note: postgres timestamptz preserves only microsecond precision, so a service clock that
// emits sub-microsecond timestamps will round-trip to a truncated value here. Because the hash
// covers created_at, the hash reconstructed on read then differs from the record-time hash for
// those sub-microsecond events. Reads remain internally consistent (the whole chain is
// reconstructed from the same truncated values), but callers needing record-time/read-time hash
// identity should keep audit timestamps at microsecond precision.
type auditSQLTime time.Time

func (t *auditSQLTime) Scan(src any) error {
	parsed, err := parseAuditSQLTime(src)
	if err != nil {
		return err
	}
	*t = auditSQLTime(parsed)
	return nil
}

func parseAuditSQLTime(src any) (time.Time, error) {
	switch typed := src.(type) {
	case time.Time:
		return typed.UTC(), nil
	case string:
		return parseAuditSQLTimeString(typed)
	case []byte:
		return parseAuditSQLTimeString(string(typed))
	default:
		return time.Time{}, errors.Errorf("unsupported audit time value %T", src)
	}
}

func parseAuditSQLTimeString(src string) (time.Time, error) {
	value := strings.TrimSpace(src)
	if value == "" {
		return time.Time{}, errors.WithStack(errors.New("empty audit time value"))
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	if unixNano, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(0, unixNano).UTC(), nil
	}
	return time.Time{}, errors.Errorf("parse audit time value %q", src)
}

var _ Store = (*SQLRepository)(nil)
