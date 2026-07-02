package persistence

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRecordStoreSQLiteRoundTrip verifies SQLite schema creation, secondary
// lookup, and JSON round trips through the shared SQL record table.
func TestRecordStoreSQLiteRoundTrip(t *testing.T) {
	db, dialect, err := OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, DialectSQLite, dialect)

	records := NewRecordStore(db, dialect)
	payload := struct {
		Name string `json:"name"`
	}{Name: "sqlite"}
	err = records.Insert(context.Background(), Record{
		Namespace:    "test.records",
		Key:          "primary",
		SecondaryKey: "secondary",
		Data:         mustJSON(t, payload),
	})
	require.NoError(t, err)

	var loaded struct {
		Name string `json:"name"`
	}
	require.NoError(t, records.GetBySecondary(context.Background(), "test.records", "secondary", &loaded))
	require.Equal(t, payload.Name, loaded.Name)
}

// TestRecordStorePostgresRoundTrip verifies PostgreSQL JSONB record behavior
// when DATABASE_URL is available for integration tests.
func TestRecordStorePostgresRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping postgres integration test")
	}

	db, dialect, err := OpenSQL("postgres", databaseURL, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, DialectPostgres, dialect)

	_, err = db.ExecContext(context.Background(), `DELETE FROM accounting_records WHERE namespace = $1`, "test.pg.records")
	require.NoError(t, err)

	records := NewRecordStore(db, dialect)
	payload := struct {
		Name string `json:"name"`
	}{Name: "postgres"}
	require.NoError(t, records.Insert(context.Background(), Record{
		Namespace:    "test.pg.records",
		Key:          "primary",
		SecondaryKey: "secondary",
		Data:         mustJSON(t, payload),
	}))

	var loaded struct {
		Name string `json:"name"`
	}
	require.NoError(t, records.Get(context.Background(), "test.pg.records", "primary", &loaded))
	require.Equal(t, payload.Name, loaded.Name)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
