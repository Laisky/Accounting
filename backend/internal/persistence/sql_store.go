package persistence

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	_ "github.com/jackc/pgx/v5/stdlib" // register the "pgx" database/sql driver
	_ "github.com/mattn/go-sqlite3"    // register the "sqlite3" database/sql driver
)

const (
	// DialectPostgres identifies PostgreSQL SQL syntax and JSONB storage.
	DialectPostgres = "postgres"
	// DialectSQLite identifies SQLite SQL syntax and JSON text storage.
	DialectSQLite = "sqlite"

	defaultSQLiteBusyTimeoutMS = 5000
	sqlOpTimeout               = 10 * time.Second
)

// Record contains one JSON-backed database row owned by a domain store.
type Record struct {
	Namespace    string
	Key          string
	ParentKey    string
	OwnerKey     string
	SecondaryKey string
	Data         []byte
}

// RecordStore persists domain records as JSON rows in a shared SQL table.
type RecordStore struct {
	db      sqlDB
	root    *sql.DB
	dialect string
}

// JoinKey returns a reversible, PostgreSQL-safe compound key for SQL records.
func JoinKey(parts ...string) string {
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		encoded = append(encoded, base64.RawURLEncoding.EncodeToString([]byte(part)))
	}
	return strings.Join(encoded, ".")
}

// OpenSQL opens a database/sql pool for the configured dialect and ensures the
// shared record schema exists.
func OpenSQL(driver string, databaseURL string, dataDir string) (*sql.DB, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(driver))
	switch normalized {
	case "postgres", "postgresql":
		if strings.TrimSpace(databaseURL) == "" {
			return nil, "", errors.WithStack(errors.New("postgres persistence requires ACCOUNTING_DATABASE_URL or DATABASE_URL"))
		}
		db, err := sql.Open("pgx", databaseURL)
		if err != nil {
			return nil, "", errors.Wrap(err, "open postgres")
		}
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(time.Hour)
		if err := pingAndMigrate(db, DialectPostgres); err != nil {
			_ = db.Close()
			return nil, "", err
		}
		return db, DialectPostgres, nil
	case "sqlite":
		sqlitePath, err := sqlitePath(databaseURL, dataDir)
		if err != nil {
			return nil, "", err
		}
		db, err := sql.Open("sqlite3", sqliteDSN(sqlitePath))
		if err != nil {
			return nil, "", errors.Wrap(err, "open sqlite")
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		if err := pingAndMigrate(db, DialectSQLite); err != nil {
			_ = db.Close()
			return nil, "", err
		}
		return db, DialectSQLite, nil
	default:
		return nil, "", errors.Errorf("unsupported sql persistence driver %q", driver)
	}
}

// NewRecordStore returns a SQL-backed JSON record store for domain packages.
func NewRecordStore(db *sql.DB, dialect string) *RecordStore {
	return &RecordStore{db: db, root: db, dialect: dialect}
}

// DB returns the underlying database/sql handle for integration tests.
func (s *RecordStore) DB() *sql.DB {
	return s.root
}

// Count receives a namespace and returns its current row count.
func (s *RecordStore) Count(ctx context.Context, namespace string) (int, error) {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	var count int
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM accounting_records WHERE namespace = ?`), namespace).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "count records")
	}
	return count, nil
}

// Insert receives a record and writes it when its primary key is absent.
func (s *RecordStore) Insert(ctx context.Context, record Record) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	query := `INSERT INTO accounting_records
		(namespace, record_key, parent_key, owner_key, secondary_key, data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ` + dataPlaceholder(s.dialect) + `, ?, ?)`
	if _, err := s.db.ExecContext(ctx, s.rebind(query), record.Namespace, record.Key, record.ParentKey,
		record.OwnerKey, record.SecondaryKey, sqlData(s.dialect, record.Data), nowValue(s.dialect), nowValue(s.dialect)); err != nil {
		return errors.Wrap(err, "insert record")
	}
	return nil
}

// Upsert receives a record and inserts or replaces its JSON payload and indexes.
func (s *RecordStore) Upsert(ctx context.Context, record Record) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	query := `INSERT INTO accounting_records
		(namespace, record_key, parent_key, owner_key, secondary_key, data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ` + dataPlaceholder(s.dialect) + `, ?, ?)
		ON CONFLICT(namespace, record_key) DO UPDATE SET
			parent_key = excluded.parent_key,
			owner_key = excluded.owner_key,
			secondary_key = excluded.secondary_key,
			data = excluded.data,
			updated_at = excluded.updated_at`
	if _, err := s.db.ExecContext(ctx, s.rebind(query), record.Namespace, record.Key, record.ParentKey,
		record.OwnerKey, record.SecondaryKey, sqlData(s.dialect, record.Data), nowValue(s.dialect), nowValue(s.dialect)); err != nil {
		return errors.Wrap(err, "upsert record")
	}
	return nil
}

// Update receives a record and replaces an existing row.
func (s *RecordStore) Update(ctx context.Context, record Record) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	query := `UPDATE accounting_records
		SET parent_key = ?, owner_key = ?, secondary_key = ?, data = ` + dataPlaceholder(s.dialect) + `, updated_at = ?
		WHERE namespace = ? AND record_key = ?`
	result, err := s.db.ExecContext(ctx, s.rebind(query),
		record.ParentKey, record.OwnerKey, record.SecondaryKey, sqlData(s.dialect, record.Data),
		nowValue(s.dialect), record.Namespace, record.Key)
	if err != nil {
		return errors.Wrap(err, "update record")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "read update count")
	}
	if affected == 0 {
		return errors.WithStack(sql.ErrNoRows)
	}
	return nil
}

// Delete removes one record by namespace and primary key.
func (s *RecordStore) Delete(ctx context.Context, namespace string, key string) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	if _, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM accounting_records WHERE namespace = ? AND record_key = ?`), namespace, key); err != nil {
		return errors.Wrap(err, "delete record")
	}
	return nil
}

// DeleteByOwner removes all records in a namespace that match an owner key.
func (s *RecordStore) DeleteByOwner(ctx context.Context, namespace string, ownerKey string) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	if _, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM accounting_records WHERE namespace = ? AND owner_key = ?`), namespace, ownerKey); err != nil {
		return errors.Wrap(err, "delete records by owner")
	}
	return nil
}

// Get decodes a record by namespace and primary key into dst.
func (s *RecordStore) Get(ctx context.Context, namespace string, key string, dst any) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	return scanJSON(s.db.QueryRowContext(ctx, s.rebind(`SELECT data FROM accounting_records WHERE namespace = ? AND record_key = ?`), namespace, key), dst)
}

// GetBySecondary decodes a record by namespace and secondary key into dst.
func (s *RecordStore) GetBySecondary(ctx context.Context, namespace string, secondaryKey string, dst any) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	return scanJSON(s.db.QueryRowContext(ctx, s.rebind(`SELECT data FROM accounting_records WHERE namespace = ? AND secondary_key = ?`), namespace, secondaryKey), dst)
}

// List decodes records matching indexes into a slice pointer.
func (s *RecordStore) List(ctx context.Context, namespace string, parentKey *string, ownerKey *string, dst any) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	query := `SELECT data FROM accounting_records WHERE namespace = ?`
	args := []any{namespace}
	if parentKey != nil {
		query += ` AND parent_key = ?`
		args = append(args, *parentKey)
	}
	if ownerKey != nil {
		query += ` AND owner_key = ?`
		args = append(args, *ownerKey)
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return errors.Wrap(err, "list records")
	}
	defer func() {
		_ = rows.Close()
	}()

	values := []json.RawMessage{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return errors.Wrap(err, "scan record")
		}
		values = append(values, json.RawMessage(data))
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "iterate records")
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return errors.Wrap(err, "encode record list")
	}
	if err := json.Unmarshal(encoded, dst); err != nil {
		return errors.Wrap(err, "decode record list")
	}
	return nil
}

// ListRecords returns raw records for a namespace.
func (s *RecordStore) ListRecords(ctx context.Context, namespace string) ([]Record, error) {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT namespace, record_key, parent_key, owner_key, secondary_key, data FROM accounting_records WHERE namespace = ?`), namespace)
	if err != nil {
		return nil, errors.Wrap(err, "list raw records")
	}
	defer func() {
		_ = rows.Close()
	}()

	records := []Record{}
	for rows.Next() {
		var record Record
		if err := rows.Scan(&record.Namespace, &record.Key, &record.ParentKey, &record.OwnerKey, &record.SecondaryKey, &record.Data); err != nil {
			return nil, errors.Wrap(err, "scan raw record")
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate raw records")
	}

	return records, nil
}

// WithTx runs fn in a database transaction.
func (s *RecordStore) WithTx(ctx context.Context, fn func(*RecordStore) error) error {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	if s.root == nil {
		return errors.WithStack(errors.New("nested record transactions are not supported"))
	}
	tx, err := s.root.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "begin transaction")
	}
	txStore := &RecordStore{db: txDB{tx: tx}, dialect: s.dialect}
	if err := fn(txStore); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "commit transaction")
	}
	return nil
}

// IncrementCounter atomically increments and returns an integer counter record.
func (s *RecordStore) IncrementCounter(ctx context.Context, namespace string, key string) (int, error) {
	var next int
	err := s.WithTx(ctx, func(tx *RecordStore) error {
		current, err := tx.Counter(ctx, namespace, key)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		next = current + 1
		data, err := json.Marshal(next)
		if err != nil {
			return errors.Wrap(err, "encode counter")
		}
		return tx.Upsert(ctx, Record{Namespace: namespace, Key: key, Data: data})
	})
	if err != nil {
		return 0, err
	}
	return next, nil
}

// Counter returns an integer counter record or zero when absent.
func (s *RecordStore) Counter(ctx context.Context, namespace string, key string) (int, error) {
	var count int
	err := s.Get(ctx, namespace, key, &count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return count, err
}

type txDB struct {
	tx *sql.Tx
}

func (t txDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t txDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t txDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

type sqlDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func pingAndMigrate(db *sql.DB, dialect string) error {
	ctx, cancel := context.WithTimeout(context.Background(), sqlOpTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return errors.Wrap(err, "ping sql database")
	}
	if dialect == DialectPostgres {
		if _, err := db.ExecContext(ctx, `SET TIME ZONE 'UTC'`); err != nil {
			return errors.Wrap(err, "set postgres utc timezone")
		}
	}
	if err := ensureRecordSchema(ctx, db, dialect); err != nil {
		return errors.Wrap(err, "ensure record schema")
	}
	return nil
}

func ensureRecordSchema(ctx context.Context, db *sql.DB, dialect string) error {
	switch dialect {
	case DialectPostgres:
		return execAll(ctx, db, []string{
			`CREATE TABLE IF NOT EXISTS accounting_records (
				namespace text NOT NULL,
				record_key text NOT NULL,
				parent_key text NOT NULL DEFAULT '',
				owner_key text NOT NULL DEFAULT '',
				secondary_key text NOT NULL DEFAULT '',
				data jsonb NOT NULL,
				created_at timestamptz NOT NULL DEFAULT now(),
				updated_at timestamptz NOT NULL DEFAULT now(),
				PRIMARY KEY (namespace, record_key)
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS accounting_records_secondary_unique
				ON accounting_records(namespace, secondary_key) WHERE secondary_key <> ''`,
			`CREATE INDEX IF NOT EXISTS accounting_records_parent_idx
				ON accounting_records(namespace, parent_key)`,
			`CREATE INDEX IF NOT EXISTS accounting_records_owner_idx
				ON accounting_records(namespace, owner_key)`,
		})
	case DialectSQLite:
		return execAll(ctx, db, []string{
			`PRAGMA foreign_keys = ON`,
			`PRAGMA journal_mode = WAL`,
			`CREATE TABLE IF NOT EXISTS accounting_records (
				namespace TEXT NOT NULL,
				record_key TEXT NOT NULL,
				parent_key TEXT NOT NULL DEFAULT '',
				owner_key TEXT NOT NULL DEFAULT '',
				secondary_key TEXT NOT NULL DEFAULT '',
				data TEXT NOT NULL CHECK (json_valid(data)),
				created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
				updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
				PRIMARY KEY (namespace, record_key)
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS accounting_records_secondary_unique
				ON accounting_records(namespace, secondary_key) WHERE secondary_key <> ''`,
			`CREATE INDEX IF NOT EXISTS accounting_records_parent_idx
				ON accounting_records(namespace, parent_key)`,
			`CREATE INDEX IF NOT EXISTS accounting_records_owner_idx
				ON accounting_records(namespace, owner_key)`,
		})
	default:
		return errors.Errorf("unsupported sql dialect %q", dialect)
	}
}

func execAll(ctx context.Context, db *sql.DB, statements []string) error {
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return errors.Wrap(err, "execute schema statement")
		}
	}
	return nil
}

func scanJSON(row *sql.Row, dst any) error {
	var data []byte
	if err := row.Scan(&data); err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return errors.Wrap(err, "decode record")
	}
	return nil
}

func sqlData(dialect string, data []byte) any {
	if dialect == DialectPostgres {
		return string(data)
	}
	return string(data)
}

func dataPlaceholder(dialect string) string {
	if dialect == DialectPostgres {
		return "?::jsonb"
	}
	return "?"
}

func (s *RecordStore) rebind(query string) string {
	if s.dialect != DialectPostgres {
		return query
	}
	var builder strings.Builder
	index := 1
	for _, char := range query {
		if char == '?' {
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(index))
			index++
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func nowValue(dialect string) any {
	if dialect == DialectSQLite {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return time.Now().UTC()
}

func withDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, sqlOpTimeout)
}

func sqlitePath(databaseURL string, dataDir string) (string, error) {
	raw := strings.TrimSpace(databaseURL)
	if raw == "" {
		raw = filepath.Join(dataDir, "accounting.sqlite3")
	}
	if strings.HasPrefix(raw, "sqlite://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", errors.Wrap(err, "parse sqlite url")
		}
		raw = parsed.Path
		if parsed.Host != "" {
			raw = filepath.Join(string(filepath.Separator)+parsed.Host, parsed.Path)
		}
	}
	absPath, err := filepath.Abs(raw)
	if err != nil {
		return "", errors.Wrap(err, "resolve sqlite path")
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return "", errors.Wrap(err, "create sqlite directory")
	}
	return absPath, nil
}

func sqliteDSN(path string) string {
	values := url.Values{}
	values.Set("_busy_timeout", strconv.Itoa(defaultSQLiteBusyTimeoutMS))
	values.Set("_journal_mode", "WAL")
	values.Set("_synchronous", "NORMAL")
	values.Set("_foreign_keys", "on")
	values.Set("_txlock", "immediate")
	values.Set("_loc", "UTC")
	return path + "?" + values.Encode()
}
