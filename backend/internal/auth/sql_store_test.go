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

// TestSQLStoreThrottlePersists verifies auth throttle state is stored in SQL rows.
func TestSQLStoreThrottlePersists(t *testing.T) {
	db, _, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	err = store.StoreLoginThrottle(context.Background(), LoginThrottle{
		Email:       "user@example.test",
		FailedCount: 6,
		LockedUntil: now.Add(time.Minute),
		UpdatedAt:   now,
	})
	require.NoError(t, err)

	reopened := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	throttle, err := reopened.LoginThrottle(context.Background(), "user@example.test")
	require.NoError(t, err)
	require.Equal(t, 6, throttle.FailedCount)
	require.Equal(t, now.Add(time.Minute), throttle.LockedUntil)
	require.Equal(t, now, throttle.UpdatedAt)
}

// TestSQLStoreDeleteSessionsByUser verifies user-scoped session deletion uses the SQL owner index.
func TestSQLStoreDeleteSessionsByUser(t *testing.T) {
	db, _, err := persistence.OpenSQL("sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := NewSQLStore(persistence.NewRecordStore(db, persistence.DialectSQLite))
	now := time.Now().UTC()
	require.NoError(t, store.StoreSession(context.Background(), "token_1", Session{
		ID:        "session_1",
		UserID:    "user_1",
		UserEmail: "one@example.test",
		Status:    UserStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}))
	require.NoError(t, store.StoreSession(context.Background(), "token_2", Session{
		ID:        "session_2",
		UserID:    "user_1",
		UserEmail: "one@example.test",
		Status:    UserStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}))
	require.NoError(t, store.StoreSession(context.Background(), "token_3", Session{
		ID:        "session_3",
		UserID:    "user_2",
		UserEmail: "two@example.test",
		Status:    UserStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}))

	require.NoError(t, store.DeleteSessionsByUser(context.Background(), "user_1"))

	_, err = store.SessionByTokenHash(context.Background(), "token_1")
	require.Error(t, err)
	_, err = store.SessionByTokenHash(context.Background(), "token_2")
	require.Error(t, err)
	session, err := store.SessionByTokenHash(context.Background(), "token_3")
	require.NoError(t, err)
	require.Equal(t, "user_2", session.UserID)
}
