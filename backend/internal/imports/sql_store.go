package imports

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Laisky/errors/v2"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

const importBatchesNS = "imports.batches"

// SQLStore persists import preview batches directly in SQL rows.
type SQLStore struct {
	records *persistence.RecordStore
}

// NewSQLStore receives a record store and returns a direct SQL import Store implementation.
func NewSQLStore(records *persistence.RecordStore) *SQLStore {
	return &SQLStore{records: records}
}

// NewPostgresStore receives a database handle and returns a direct SQL import store.
func NewPostgresStore(db *sql.DB) (*SQLStore, error) {
	return NewSQLStore(persistence.NewRecordStore(db, persistence.DialectPostgres)), nil
}

// NewSQLiteStore receives a database handle and returns a direct SQL import store.
func NewSQLiteStore(db *sql.DB) (*SQLStore, error) {
	return NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite)), nil
}

// SaveBatch receives an import batch and stores it by id and source hash.
func (s *SQLStore) SaveBatch(ctx context.Context, batch Batch) (Batch, error) {
	batch = cloneBatch(batch)
	if err := s.records.Upsert(ctx, batchRecord(batch)); err != nil {
		return Batch{}, errors.Wrap(err, "save import batch")
	}
	return cloneBatch(batch), nil
}

// SaveBatchIfAbsent atomically stores a batch only when no batch already exists
// for the same owner/source/hash, returning the existing batch otherwise.
func (s *SQLStore) SaveBatchIfAbsent(ctx context.Context, batch Batch) (Batch, bool, error) {
	key := batchLookupKey(batch.UserID, batch.Source, batch.SourceHash)
	var existing Batch
	err := s.records.GetBySecondary(ctx, importBatchesNS, key, &existing)
	if err == nil {
		return cloneBatch(existing), false, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Batch{}, false, errors.Wrap(err, "load import batch by hash")
	}

	batch = cloneBatch(batch)
	if err := s.records.Insert(ctx, batchRecord(batch)); err != nil {
		if lookupErr := s.records.GetBySecondary(ctx, importBatchesNS, key, &existing); lookupErr == nil {
			return cloneBatch(existing), false, nil
		}
		return Batch{}, false, errors.Wrap(err, "insert import batch")
	}
	return cloneBatch(batch), true, nil
}

// Batch receives owner and batch id values and returns the matching batch.
func (s *SQLStore) Batch(ctx context.Context, userID string, batchID string) (Batch, error) {
	var batch Batch
	if err := s.records.Get(ctx, importBatchesNS, batchID, &batch); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
		}
		return Batch{}, errors.Wrap(err, "load import batch")
	}
	if batch.UserID != userID {
		return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
	}
	return cloneBatch(batch), nil
}

// BatchByHash receives owner, source, and hash values and returns the matching batch.
func (s *SQLStore) BatchByHash(ctx context.Context, userID string, source string, sourceHash string) (Batch, error) {
	var batch Batch
	if err := s.records.GetBySecondary(ctx, importBatchesNS, batchLookupKey(userID, source, sourceHash), &batch); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Batch{}, errors.Wrap(ErrNotFound, "import batch not found")
		}
		return Batch{}, errors.Wrap(err, "load import batch by hash")
	}
	return cloneBatch(batch), nil
}

func batchRecord(batch Batch) persistence.Record {
	data, err := json.Marshal(batch)
	if err != nil {
		panic(err)
	}
	return persistence.Record{
		Namespace:    importBatchesNS,
		Key:          batch.ID,
		OwnerKey:     batch.UserID,
		SecondaryKey: batchLookupKey(batch.UserID, batch.Source, batch.SourceHash),
		Data:         data,
	}
}

func batchLookupKey(userID string, source string, sourceHash string) string {
	return persistence.JoinKey(userID, source, sourceHash)
}
