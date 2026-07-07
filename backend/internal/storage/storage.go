// Package storage owns SQL connection setup, schema migrations, and transaction boundaries.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Laisky/errors/v2"
	_ "github.com/jackc/pgx/v5/stdlib" // register the "pgx" database/sql driver
	_ "github.com/mattn/go-sqlite3"    // register the "sqlite3" database/sql driver
	"github.com/pressly/goose/v3"
)

const (
	// DialectPostgres identifies PostgreSQL SQL syntax.
	DialectPostgres = "postgres"
	// DialectSQLite identifies SQLite SQL syntax.
	DialectSQLite = "sqlite"

	defaultSQLiteBusyTimeoutMS = 5000
	sqlOpTimeout               = 10 * time.Second
)

//go:embed migrations/postgres/*.sql migrations/sqlite/*.sql
var migrationFS embed.FS

var gooseMu sync.Mutex

// DB wraps a database/sql pool with the selected SQL dialect.
type DB struct {
	conn    *sql.DB
	dialect string
}

// DBTX is the minimal SQL execution surface shared by pools and transactions.
type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Open receives driver settings and returns a configured SQL pool without running migrations.
func Open(ctx context.Context, driver string, databaseURL string, dataDir string) (*DB, error) {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	normalized := strings.ToLower(strings.TrimSpace(driver))
	switch normalized {
	case "postgres", "postgresql":
		if strings.TrimSpace(databaseURL) == "" {
			return nil, errors.WithStack(errors.New("postgres storage requires ACCOUNTING_DATABASE_URL or DATABASE_URL"))
		}
		db, err := sql.Open("pgx", databaseURL)
		if err != nil {
			return nil, errors.Wrap(err, "open postgres")
		}
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(time.Hour)
		if err := pingAndConfigure(ctx, db, DialectPostgres); err != nil {
			_ = db.Close()
			return nil, err
		}
		return &DB{conn: db, dialect: DialectPostgres}, nil
	case "sqlite":
		path, err := sqlitePath(databaseURL, dataDir)
		if err != nil {
			return nil, err
		}
		db, err := sql.Open("sqlite3", sqliteDSN(path))
		if err != nil {
			return nil, errors.Wrap(err, "open sqlite")
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		if err := pingAndConfigure(ctx, db, DialectSQLite); err != nil {
			_ = db.Close()
			return nil, err
		}
		return &DB{conn: db, dialect: DialectSQLite}, nil
	default:
		return nil, errors.Errorf("unsupported storage driver %q", driver)
	}
}

// SQLDB returns the underlying database/sql pool.
func (db *DB) SQLDB() *sql.DB {
	if db == nil {
		return nil
	}
	return db.conn
}

// Dialect returns the selected SQL dialect.
func (db *DB) Dialect() string {
	if db == nil {
		return ""
	}
	return db.dialect
}

// Close closes the underlying database/sql pool.
func (db *DB) Close() error {
	if db == nil || db.conn == nil {
		return nil
	}
	if err := db.conn.Close(); err != nil {
		return errors.Wrap(err, "close storage database")
	}
	return nil
}

// Migrate applies every embedded migration for the selected dialect.
func (db *DB) Migrate(ctx context.Context) error {
	if db == nil || db.conn == nil {
		return errors.WithStack(errors.New("storage database is nil"))
	}
	return Migrate(ctx, db.conn, db.dialect)
}

// WithTx runs fn inside one SQL transaction, rolling back when fn returns an error.
func (db *DB) WithTx(ctx context.Context, fn func(DBTX) error) error {
	if db == nil || db.conn == nil {
		return errors.WithStack(errors.New("storage database is nil"))
	}
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "begin transaction")
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "commit transaction")
	}
	return nil
}

// Migrate receives a SQL pool and dialect and applies embedded goose migrations.
func Migrate(ctx context.Context, db *sql.DB, dialect string) error {
	if db == nil {
		return errors.WithStack(errors.New("sql database is nil"))
	}
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	gooseDialect, migrationDir, err := gooseSettings(dialect)
	if err != nil {
		return err
	}

	gooseMu.Lock()
	defer gooseMu.Unlock()
	goose.SetBaseFS(migrationFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect(gooseDialect); err != nil {
		return errors.Wrap(err, "set goose dialect")
	}
	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		return errors.Wrap(err, "run migrations")
	}
	return nil
}

// Rebind converts question-mark placeholders to dialect-specific placeholders.
func Rebind(dialect string, query string) string {
	if dialect != DialectPostgres {
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

func pingAndConfigure(ctx context.Context, db *sql.DB, dialect string) error {
	if err := db.PingContext(ctx); err != nil {
		return errors.Wrap(err, "ping sql database")
	}
	switch dialect {
	case DialectPostgres:
		if _, err := db.ExecContext(ctx, `SET TIME ZONE 'UTC'`); err != nil {
			return errors.Wrap(err, "set postgres utc timezone")
		}
	case DialectSQLite:
		if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			return errors.Wrap(err, "enable sqlite foreign keys")
		}
		if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL`); err != nil {
			return errors.Wrap(err, "enable sqlite wal")
		}
	default:
		return errors.Errorf("unsupported sql dialect %q", dialect)
	}
	return nil
}

func gooseSettings(dialect string) (string, string, error) {
	switch dialect {
	case DialectPostgres:
		return "postgres", "migrations/postgres", nil
	case DialectSQLite:
		return "sqlite3", "migrations/sqlite", nil
	default:
		return "", "", errors.Errorf("unsupported sql dialect %q", dialect)
	}
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
	values.Set("_foreign_keys", "on")
	return "file:" + path + "?" + values.Encode()
}

func withDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, sqlOpTimeout)
}
