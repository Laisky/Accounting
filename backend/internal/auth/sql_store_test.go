package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/persistence"
)

// TestSQLStorePersistsUserAcrossStoreInstances verifies user writes are visible
// to a fresh SQL store without an in-memory flush step.
func TestSQLStorePersistsUserAcrossStoreInstances(t *testing.T) {
	db, _, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	now := time.Now().UTC()
	user := UserRecord{
		User: User{
			ID:            "user_sql",
			Email:         "sql@example.test",
			Status:        UserStatusActive,
			EmailVerified: true,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		PasswordHash: "hash",
	}
	_, err = store.CreateUser(context.Background(), user)
	require.NoError(t, err)

	reopened := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	loaded, err := reopened.UserByEmail(context.Background(), user.Email)
	require.NoError(t, err)
	require.Equal(t, user.ID, loaded.ID)
	require.Equal(t, user.PasswordHash, loaded.PasswordHash)
	require.Equal(t, user.CreatedAt, loaded.CreatedAt)
	require.Equal(t, user.UpdatedAt, loaded.UpdatedAt)
}

// TestSQLStoreCountersPersist verifies auth failure counters are stored in SQL rows.
func TestSQLStoreCountersPersist(t *testing.T) {
	db, _, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	count, err := store.IncrementFailedLogin(context.Background(), "user@example.test")
	require.NoError(t, err)
	require.Equal(t, 1, count)
	count, err = store.IncrementFailedLogin(context.Background(), "user@example.test")
	require.NoError(t, err)
	require.Equal(t, 2, count)

	reopened := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	count, err = reopened.FailedLoginCount(context.Background(), "user@example.test")
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
