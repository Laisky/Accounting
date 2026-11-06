package imports

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// importBatchColumns lists the import_batches columns in a fixed order shared by reads and writes.
const importBatchColumns = `id, user_id, source, filename, content_type, source_hash, parser_version, ` +
	`status, detected_schema, detected, error_count, warning_count, applied_book_id, ` +
	`applied_entry_ids, applied_skipped_rows, applied_at, created_at, updated_at`

// SQLRepository persists import preview batches in the relational core schema.
type SQLRepository struct {
	db      *storage.DB
	dialect string
}

// NewSQLRepository receives a migrated storage database and returns a relational import Store.
func NewSQLRepository(db *storage.DB) (*SQLRepository, error) {
	if db == nil || db.SQLDB() == nil {
		return nil, errors.WithStack(errors.New("storage database is nil"))
	}
	return &SQLRepository{db: db, dialect: db.Dialect()}, nil
}

// SaveBatch receives an import batch and upserts it along with its preview rows in one transaction.
func (s *SQLRepository) SaveBatch(ctx context.Context, batch Batch) (Batch, error) {
	batch = cloneBatch(batch)
	values, err := batchRowValues(batch)
	if err != nil {
		return Batch{}, err
	}
	if err := s.db.WithTx(ctx, func(tx storage.DBTX) error {
		if err := s.upsertBatchRow(ctx, tx, values); err != nil {
			return err
		}
		return s.replaceRows(ctx, tx, batch)
	}); err != nil {
		return Batch{}, err
	}
	return cloneBatch(batch), nil
}

// SaveBatchIfAbsent atomically stores a batch only when no batch already exists for the same
// owner/source/hash, returning the existing batch otherwise. It relies on the
// UNIQUE(user_id, source_hash) constraint to close the check-then-save race.
func (s *SQLRepository) SaveBatchIfAbsent(ctx context.Context, batch Batch) (Batch, bool, error) {
	existing, err := s.batchByHash(ctx, batch.UserID, batch.Source, batch.SourceHash)
	if err == nil {
		return cloneBatch(existing), false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Batch{}, false, errors.Wrap(err, "load import batch by hash")
	}

	batch = cloneBatch(batch)
	values, err := batchRowValues(batch)
	if err != nil {
		return Batch{}, false, err
	}
	insertErr := s.db.WithTx(ctx, func(tx storage.DBTX) error {
		if err := s.insertBatchRow(ctx, tx, values); err != nil {
			return err
		}
		return s.insertRows(ctx, tx, batch)
	})
	if insertErr != nil {
		if existing, lookupErr := s.batchByHash(ctx, batch.UserID, batch.Source, batch.SourceHash); lookupErr == nil {
			return cloneBatch(existing), false, nil
		}
		return Batch{}, false, errors.Wrap(insertErr, "insert import batch")
	}
	return cloneBatch(batch), true, nil
}

// Batch receives owner and batch id values and returns the matching batch with its rows.
func (s *SQLRepository) Batch(ctx context.Context, userID string, batchID string) (Batch, error) {
	query := storage.Rebind(s.dialect, `SELECT `+importBatchColumns+` FROM import_batches WHERE id = ?`)
	row := s.db.SQLDB().QueryRowContext(ctx, query, batchID) //nolint:gosec // column list is a compile-time constant; every value is parameterized
	batch, err := s.scanBatchRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
		}
		return Batch{}, errors.Wrap(err, "load import batch")
	}
	if batch.UserID != userID {
		return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
	}
	rows, err := s.loadRows(ctx, batch.ID)
	if err != nil {
		return Batch{}, err
	}
	batch.Rows = rows
	return cloneBatch(batch), nil
}

// BatchByHash receives owner, source, and hash values and returns the matching batch with its rows.
func (s *SQLRepository) BatchByHash(ctx context.Context, userID string, source string, sourceHash string) (Batch, error) {
	batch, err := s.batchByHash(ctx, userID, source, sourceHash)
	if err != nil {
		return Batch{}, err
	}
	return cloneBatch(batch), nil
}

// batchByHash loads a batch and its rows keyed on owner/source/hash, returning ErrNotFound when absent.
func (s *SQLRepository) batchByHash(ctx context.Context, userID string, source string, sourceHash string) (Batch, error) {
	query := storage.Rebind(s.dialect, `SELECT `+importBatchColumns+` FROM import_batches WHERE user_id = ? AND source = ? AND source_hash = ?`)
	row := s.db.SQLDB().QueryRowContext(ctx, query, userID, source, sourceHash) //nolint:gosec // column list is a compile-time constant; every value is parameterized
	batch, err := s.scanBatchRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
		}
		return Batch{}, errors.Wrap(err, "load import batch by hash")
	}
	rows, err := s.loadRows(ctx, batch.ID)
	if err != nil {
		return Batch{}, err
	}
	batch.Rows = rows
	return batch, nil
}

// ClaimForApply atomically transitions a batch from preview to applying with a single
// conditional UPDATE. Exactly one of two concurrent claims affects a row, so the loser sees
// zero rows affected and receives ErrConflict — closing the concurrent double-apply window.
func (s *SQLRepository) ClaimForApply(ctx context.Context, userID string, batchID string) (Batch, error) {
	query := storage.Rebind(s.dialect, `UPDATE import_batches SET status = 'applying', updated_at = ? WHERE id = ? AND user_id = ? AND status = 'preview'`)
	res, err := s.db.SQLDB().ExecContext(ctx, query, time.Now().UTC(), batchID, userID)
	if err != nil {
		return Batch{}, errors.Wrap(err, "claim import batch")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Batch{}, errors.Wrap(err, "claim rows affected")
	}
	if affected == 0 {
		existing, loadErr := s.Batch(ctx, userID, batchID)
		if loadErr != nil {
			return Batch{}, loadErr
		}
		return Batch{}, errors.Wrapf(ErrConflict, "import batch is %s", existing.Status)
	}
	return s.Batch(ctx, userID, batchID)
}

// FinalizeApplied atomically transitions a claimed (applying) batch to applied with a single
// conditional UPDATE, recording the commit metadata.
func (s *SQLRepository) FinalizeApplied(ctx context.Context, request MarkAppliedRequest) (Batch, error) {
	entryIDs, err := json.Marshal(request.EntryIDs)
	if err != nil {
		return Batch{}, errors.Wrap(err, "encode applied entry ids")
	}
	skipped, err := json.Marshal(cloneAppliedSkippedRows(request.SkippedRows))
	if err != nil {
		return Batch{}, errors.Wrap(err, "encode applied skipped rows")
	}
	now := time.Now().UTC()
	query := storage.Rebind(s.dialect, `UPDATE import_batches SET status = 'applied', applied_book_id = ?, applied_entry_ids = ?, applied_skipped_rows = ?, applied_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND status = 'applying'`)
	res, err := s.db.SQLDB().ExecContext(ctx, query, nullString(request.BookID), string(entryIDs), string(skipped), now, now, request.BatchID, request.Actor.UserID) //nolint:gosec // static query; every value is parameterized
	if err != nil {
		return Batch{}, errors.Wrap(err, "finalize import batch")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Batch{}, errors.Wrap(err, "finalize rows affected")
	}
	if affected == 0 {
		existing, loadErr := s.Batch(ctx, request.Actor.UserID, request.BatchID)
		if loadErr != nil {
			return Batch{}, loadErr
		}
		return Batch{}, errors.Wrapf(ErrConflict, "import batch is %s", existing.Status)
	}
	return s.Batch(ctx, request.Actor.UserID, request.BatchID)
}

// RevertToPreview atomically returns an applying batch to preview as the failed-apply
// compensating action; it is idempotent (zero rows affected is not an error).
func (s *SQLRepository) RevertToPreview(ctx context.Context, userID string, batchID string) error {
	query := storage.Rebind(s.dialect, `UPDATE import_batches SET status = 'preview', updated_at = ? WHERE id = ? AND user_id = ? AND status = 'applying'`)
	if _, err := s.db.SQLDB().ExecContext(ctx, query, time.Now().UTC(), batchID, userID); err != nil {
		return errors.Wrap(err, "revert import batch")
	}
	return nil
}

// upsertBatchRow inserts or replaces the scalar batch row, keeping preview rows in import_rows.
func (s *SQLRepository) upsertBatchRow(ctx context.Context, tx storage.DBTX, values []any) error {
	_, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO import_batches (`+importBatchColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			user_id = excluded.user_id,
			source = excluded.source,
			filename = excluded.filename,
			content_type = excluded.content_type,
			source_hash = excluded.source_hash,
			parser_version = excluded.parser_version,
			status = excluded.status,
			detected_schema = excluded.detected_schema,
			detected = excluded.detected,
			error_count = excluded.error_count,
			warning_count = excluded.warning_count,
			applied_book_id = excluded.applied_book_id,
			applied_entry_ids = excluded.applied_entry_ids,
			applied_skipped_rows = excluded.applied_skipped_rows,
			applied_at = excluded.applied_at,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at`), values...)
	if err != nil {
		return errors.Wrap(err, "upsert import batch")
	}
	return nil
}

// insertBatchRow inserts the scalar batch row and lets the UNIQUE(user_id, source_hash) constraint reject duplicates.
func (s *SQLRepository) insertBatchRow(ctx context.Context, tx storage.DBTX, values []any) error {
	_, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
		INSERT INTO import_batches (`+importBatchColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`), values...)
	if err != nil {
		return errors.Wrap(err, "insert import batch")
	}
	return nil
}

// replaceRows deletes then reinserts the preview rows for a batch so SaveBatch stays authoritative.
func (s *SQLRepository) replaceRows(ctx context.Context, tx storage.DBTX, batch Batch) error {
	if _, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
		DELETE FROM import_rows WHERE batch_id = ?`), batch.ID); err != nil {
		return errors.Wrap(err, "delete import rows")
	}
	return s.insertRows(ctx, tx, batch)
}

// insertRows persists each preview row into import_rows, one row per PreviewRow.
func (s *SQLRepository) insertRows(ctx context.Context, tx storage.DBTX, batch Batch) error {
	for _, row := range batch.Rows {
		data, err := json.Marshal(row)
		if err != nil {
			return errors.Wrap(err, "encode import row")
		}
		if _, err := tx.ExecContext(ctx, storage.Rebind(s.dialect, `
			INSERT INTO import_rows (batch_id, row_number, data, error_count, created_at)
			VALUES (?, ?, ?, ?, ?)`), batch.ID, row.RowNumber, string(data), len(row.Errors), batch.CreatedAt); err != nil {
			return errors.Wrap(err, "insert import row")
		}
	}
	return nil
}

// loadRows returns the preview rows for a batch ordered by row_number.
func (s *SQLRepository) loadRows(ctx context.Context, batchID string) ([]PreviewRow, error) {
	query := storage.Rebind(s.dialect, `SELECT data FROM import_rows WHERE batch_id = ? ORDER BY row_number ASC`)
	rows, err := s.db.SQLDB().QueryContext(ctx, query, batchID) //nolint:gosec // static query; batch id is parameterized
	if err != nil {
		return nil, errors.Wrap(err, "list import rows")
	}
	defer func() { _ = rows.Close() }()

	previews := []PreviewRow{}
	for rows.Next() {
		var data jsonRaw
		if err := rows.Scan(&data); err != nil {
			return nil, errors.Wrap(err, "scan import row")
		}
		var preview PreviewRow
		if err := json.Unmarshal(data, &preview); err != nil {
			return nil, errors.Wrap(err, "decode import row")
		}
		previews = append(previews, preview)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate import rows")
	}
	return previews, nil
}

// scanRow abstracts *sql.Row and *sql.Rows for shared batch scanning.
type scanRow interface {
	Scan(dest ...any) error
}

// scanBatchRow scans one import_batches row into a Batch without its preview rows.
func (s *SQLRepository) scanBatchRow(row scanRow) (Batch, error) {
	var batch Batch
	var detectedSchema jsonRaw
	var detected jsonRaw
	var appliedEntryIDs jsonRaw
	var appliedSkippedRows jsonRaw
	var appliedBookID sql.NullString
	var appliedAt importNullableTime
	if err := row.Scan(
		&batch.ID, &batch.UserID, &batch.Source, &batch.Filename, &batch.ContentType, &batch.SourceHash,
		&batch.ParserVersion, &batch.Status, &detectedSchema, &detected, &batch.ErrorCount, &batch.WarningCount,
		&appliedBookID, &appliedEntryIDs, &appliedSkippedRows, &appliedAt,
		(*importSQLTime)(&batch.CreatedAt), (*importSQLTime)(&batch.UpdatedAt),
	); err != nil {
		return Batch{}, err
	}

	if err := json.Unmarshal(detectedSchema, &batch.DetectedSchema); err != nil {
		return Batch{}, errors.Wrap(err, "decode detected schema")
	}
	if err := json.Unmarshal(detected, &batch.Detected); err != nil {
		return Batch{}, errors.Wrap(err, "decode detected values")
	}
	if err := json.Unmarshal(appliedEntryIDs, &batch.AppliedEntryIDs); err != nil {
		return Batch{}, errors.Wrap(err, "decode applied entry ids")
	}
	if err := json.Unmarshal(appliedSkippedRows, &batch.AppliedSkippedRows); err != nil {
		return Batch{}, errors.Wrap(err, "decode applied skipped rows")
	}
	batch.AppliedBookID = appliedBookID.String
	if appliedAt.Valid {
		applied := appliedAt.Time.UTC()
		batch.AppliedAt = &applied
	}
	return batch, nil
}

// batchRowValues returns the ordered import_batches column values for one batch.
func batchRowValues(batch Batch) ([]any, error) {
	detectedSchema, err := json.Marshal(batch.DetectedSchema)
	if err != nil {
		return nil, errors.Wrap(err, "encode detected schema")
	}
	detected, err := json.Marshal(batch.Detected)
	if err != nil {
		return nil, errors.Wrap(err, "encode detected values")
	}
	appliedEntryIDs, err := json.Marshal(batch.AppliedEntryIDs)
	if err != nil {
		return nil, errors.Wrap(err, "encode applied entry ids")
	}
	appliedSkippedRows, err := json.Marshal(batch.AppliedSkippedRows)
	if err != nil {
		return nil, errors.Wrap(err, "encode applied skipped rows")
	}
	var appliedAt any
	if batch.AppliedAt != nil {
		appliedAt = batch.AppliedAt.UTC()
	}
	return []any{
		batch.ID, batch.UserID, batch.Source, batch.Filename, batch.ContentType, batch.SourceHash,
		batch.ParserVersion, string(batch.Status), string(detectedSchema), string(detected),
		batch.ErrorCount, batch.WarningCount, nullString(batch.AppliedBookID),
		string(appliedEntryIDs), string(appliedSkippedRows), appliedAt,
		batch.CreatedAt.UTC(), batch.UpdatedAt.UTC(),
	}, nil
}

// nullString returns a NULL-aware string parameter, treating empty as SQL NULL.
func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// jsonRaw scans a JSON/JSONB column into raw bytes regardless of the driver's native representation.
type jsonRaw []byte

func (j *jsonRaw) Scan(src any) error {
	switch typed := src.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append([]byte(nil), typed...)
	case string:
		*j = []byte(typed)
	default:
		return errors.Errorf("unsupported json source %T", src)
	}
	return nil
}

// importSQLTime scans a timestamp column stored as time.Time, string, or bytes into UTC.
type importSQLTime time.Time

func (t *importSQLTime) Scan(src any) error {
	parsed, err := parseImportSQLTime(src)
	if err != nil {
		return err
	}
	*t = importSQLTime(parsed)
	return nil
}

// importNullableTime scans an optional timestamp column, tracking SQL NULL separately.
type importNullableTime struct {
	Time  time.Time
	Valid bool
}

func (t *importNullableTime) Scan(src any) error {
	if src == nil {
		t.Time = time.Time{}
		t.Valid = false
		return nil
	}
	parsed, err := parseImportSQLTime(src)
	if err != nil {
		return err
	}
	t.Time = parsed
	t.Valid = true
	return nil
}

func parseImportSQLTime(src any) (time.Time, error) {
	switch typed := src.(type) {
	case time.Time:
		return typed.UTC(), nil
	case string:
		return parseImportSQLTimeString(typed)
	case []byte:
		return parseImportSQLTimeString(string(typed))
	default:
		return time.Time{}, errors.Errorf("unsupported time source %T", src)
	}
}

func parseImportSQLTimeString(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, errors.Wrapf(lastErr, "parse sql time %q", value)
}

var _ Store = (*SQLRepository)(nil)
